package transformers

import (
	"errors"
	"fmt"
	"gwEmu/pkg/config"
	"gwEmu/pkg/resource"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	apimachineryResource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var allowedSelectors = []string{"stress"}

func makeCombinedErrorMessage(errs []error) string {
	msgs := make([]string, len(errs))
	for i, err := range errs {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, ", ")
}

func Transform(resources []resource.Resource) ([]runtime.Object, error) {
	transformed := make([]runtime.Object, 0)
	errors := make([]error, 0)
	for _, res := range resources {
		switch {
		case res.GKV.Group == "apps" && res.GKV.Version == "v1" && res.GKV.Kind == "Deployment":
			t, err := transformDeployment(res.Obj.(*appsV1.Deployment))
			if err != nil {
				errors = append(errors, err)
			}
			transformed = append(transformed, t)
		default:
			transformed = append(transformed, res.Obj)
			errors = append(errors, fmt.Errorf(
				"do not know how to handle %s/%s/%s",
				res.GKV.Group, res.GKV.Version, res.GKV.Kind,
			))
		}
	}
	if len(errors) > 0 {
		return transformed, fmt.Errorf("some manfiests where not fully transformed: %s", makeCombinedErrorMessage(errors))

	}

	return transformed, nil
}

func isAllowedSelector(value string) bool {
	for _, s := range allowedSelectors {
		if s == value {
			return true
		}
	}
	return false
}

func extractFromLabels(labels map[string]string) map[string]map[string]string {
	values := make(map[string]map[string]string)

	var prefix string = "gwEmu"
	prefixSuffix, err := config.GetConifg[string]("prefix-suffix")
	if err != nil {
		if errors.Is(err, config.TypeConvError{}) {
			log.Error().Msg("Failed to convert")
		} else if errors.Is(err, config.Missing{}) {
			log.Error().Msg("No prefix-suffix not set falling back to no gwEmu")
		}
	} else {
		prefix = fmt.Sprintf("gwEmu-%s", *prefixSuffix)
	}
	prefixLen := len(strings.Split(prefix, "-"))

	for key, value := range labels {
		if strings.HasPrefix(key, prefix) {
			parts := strings.Split(key, "-")
			if !isAllowedSelector(parts[prefixLen]) {
				continue
			}
			prefixValue := parts[prefixLen]
			if _, ok := values[prefixValue]; !ok {
				values[prefixValue] = make(map[string]string)
			}
			values[prefixValue][strings.Join(parts[prefixLen+1:], "-")] = value
		}
	}
	return values
}

func getContainters(selector string, values map[string]string) ([]coreV1.Container, error) {
	var (
		repeats int
		err     error
	)
	repeatsStr, ok := values["repeats"]
	if ok {
		repeats, err = strconv.Atoi(repeatsStr)
		if err != nil {
			return []coreV1.Container{}, err
		}
	} else {
		repeats = 1
	}

	env := make([]coreV1.EnvVar, 0)
	for name, value := range values {
		// Maybe I should just pass this through aswell?
		if name == "repeats" {
			continue
		}
		env = append(env, coreV1.EnvVar{Name: name, Value: value})
	}

	var container *coreV1.Container
	if selector == "stress" {
		port := 8080
		env = append(env, coreV1.EnvVar{Name: "LISTEN_PORT", Value: fmt.Sprintf("%d", port)})
		env = append(env, coreV1.EnvVar{Name: "LISTEN", Value: "1"})

		container = &coreV1.Container{
			Image:           "ghcr.io/abraham2512/fedora-stress-ng:master",
			ImagePullPolicy: coreV1.PullAlways,
			LivenessProbe: &coreV1.Probe{
				ProbeHandler: coreV1.ProbeHandler{
					HTTPGet: &coreV1.HTTPGetAction{
						Path: "/healthz",
						Port: intstr.IntOrString{IntVal: int32(port)},
					},
				},
			},
			Resources: coreV1.ResourceRequirements{
				Limits: coreV1.ResourceList{
					coreV1.ResourceCPU:    apimachineryResource.MustParse("1000m"),
					coreV1.ResourceMemory: apimachineryResource.MustParse("1024Mi"),
				},
				Requests: coreV1.ResourceList{
					coreV1.ResourceCPU:    apimachineryResource.MustParse("1000m"),
					coreV1.ResourceMemory: apimachineryResource.MustParse("1024Mi"),
				},
			},
			Env: env,
		}
	}
	if container == nil {
		return []coreV1.Container{}, fmt.Errorf("missing container def for selector '%s'", selector)
	}
	// log.Debug().Msg(fmt.Sprintf("number of repeats %d", repeats))
	list := make([]coreV1.Container, repeats)
	for i := 0; i < repeats; i++ {
		list[i] = *container
	}
	// log.Debug().Msg(fmt.Sprintf("list of containers %v", list))
	return list, nil
}

// Check if b can fit within the limits of a
func compareContainers(a, b coreV1.Container) bool {
	log.Debug().Msgf("%v  |  %v\n", a.Resources.Limits.Cpu(), b.Resources.Limits.Cpu())
	log.Debug().Msgf("%v  |  %v\n", a.Resources.Limits.Memory(), b.Resources.Limits.Memory())
	log.Debug().Msgf("%v  |  %v\n", a.Resources.Requests.Cpu(), b.Resources.Requests.Cpu())
	log.Debug().Msgf("%v  |  %v\n", a.Resources.Requests.Memory(), b.Resources.Requests.Memory())

	if a.Resources.Limits.Cpu().Cmp(*b.Resources.Limits.Cpu()) == -1 {
		return false
	}
	if a.Resources.Limits.Memory().Cmp(*b.Resources.Limits.Memory()) == -1 {
		return false
	}
	return true
}

func reconsileContainers(current, wanted []coreV1.Container) ([]coreV1.Container, error) {
	replaced := make(map[int]bool)
	for _, container := range wanted {
		found := false
		for i, comparison := range current {
			if _, ok := replaced[i]; ok {
				continue
			}
			if compareContainers(comparison, container) {
				// replace values
				comparison.Image = container.Image
				comparison.ImagePullPolicy = container.ImagePullPolicy
				comparison.LivenessProbe = container.LivenessProbe
				comparison.Env = container.Env
				current[i] = comparison
				replaced[i] = true
				found = true
				break
			}
		}
		if !found {
			return current, fmt.Errorf("failed to find container to replace")
		}
	}
	return current, nil
}

func transformDeployment(deployment *appsV1.Deployment) (*appsV1.Deployment, error) {
	result := deployment.DeepCopy()
	containers := make([]coreV1.Container, 0)
	errs := make([]error, 0)
	for selector, values := range extractFromLabels(deployment.ObjectMeta.Labels) {
		selected, err := getContainters(selector, values)
		if err != nil {
			errs = append(errs, err)
		}
		containers = append(containers, selected...)
	}
	if len(errs) > 0 {
		return result, errors.New(makeCombinedErrorMessage(errs))
	}

	if len(deployment.Spec.Template.Spec.Containers) < len(containers) {
		return nil, fmt.Errorf(
			"not enough containers in spec %d < %d",
			len(deployment.Spec.Template.Spec.Containers),
			len(containers),
		)
	}

	replacedContainers, err := reconsileContainers(result.Spec.Template.Spec.Containers, containers)
	if err != nil {
		return result, err
	}
	result.Spec.Template.Spec.Containers = replacedContainers
	return result, nil
}
