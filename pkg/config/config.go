package config

import (
	"fmt"
)

var config map[string]any

func init() {
	config = make(map[string]any)
}

type Missing struct {
	msg string
}

func (m Missing) Error() string {
	return m.msg
}

type TypeConvError struct {
	msg string
}

func (t TypeConvError) Error() string {
	return t.msg
}

func GetConifg[T any](key string) (*T, error) {
	raw, ok := config[key]
	if !ok {
		return nil, Missing{msg: fmt.Sprintf("not found value for key: %s", key)}
	}
	v, ok := raw.(T)
	if !ok {
		return nil, TypeConvError{msg: "could not convert to type expected"}
	}
	return &v, nil
}

func SetConfig(key string, value any) {
	config[key] = value
}
