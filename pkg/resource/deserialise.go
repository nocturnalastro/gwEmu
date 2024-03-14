package resource

import (
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

type Resource struct {
	Obj runtime.Object
	GKV *schema.GroupVersionKind
}

func ParseResourceFile(fname string) []Resource {
	data, err := os.ReadFile(fname)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to open %s", fname)
	}

	decoder := scheme.Codecs.UniversalDeserializer()

	streams := strings.Split(string(data), "---")
	result := make([]Resource, len(streams))

	for i, resourceYAML := range streams {
		if resourceYAML == "" {
			continue
		}

		obj, gvk, err := decoder.Decode([]byte(resourceYAML), nil, nil)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode resource YAML")
			continue
		}
		result[i] = Resource{Obj: obj, GKV: gvk}
	}
	return result
}
