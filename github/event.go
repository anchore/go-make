package github

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

// Environment variable reference:
// https://docs.github.com/en/actions/reference/workflows-and-actions/variables#default-environment-variables

type Event struct {
	ApiURL      string      `env:"GITHUB_API_URL"`
	Token       string      `env:"GITHUB_TOKEN"`
	Type        string      `env:"GITHUB_EVENT_NAME"`
	Ref         string      `env:"GITHUB_REF"`
	Actor       string      `env:"GITHUB_ACTOR"`
	Owner       string      `env:"GITHUB_REPOSITORY_OWNER"`
	Repo        string      `env:"GITHUB_REPOSITORY"` // Repo is the full name of the repository, including the owner
	SHA         string      `env:"GITHUB_SHA"`
	Workflow    string      `env:"GITHUB_WORKFLOW"`
	RunID       int64       `env:"GITHUB_RUN_ID"`
	RunNumber   string      `env:"GITHUB_RUN_NUMBER"`
	Job         string      `env:"GITHUB_JOB"`
	Step        string      `env:"GITHUB_STEP"`
	Action      string      `env:"GITHUB_ACTION"`
	PullRequest PullRequest `json:"pull_request"`
}

func (e Event) IsPullRequest() bool {
	return e.PullRequest != PullRequest{}
}

// Payload returns the current event payload
func Payload() Event {
	out := Event{
		Token: authTokenFromEnvFile(),
	}
	envLoad(&out)
	ciEventFile := os.Getenv("GITHUB_EVENT_PATH")
	if ciEventFile != "" {
		f := lang.Return(os.Open(ciEventFile))
		defer lang.Close(f, ciEventFile)
		err := json.NewDecoder(f).Decode(&out)
		if err == nil {
			if config.Debug {
				log.Debug("event:\n%s", log.FormatJSON(string(lang.Continue(os.ReadFile(ciEventFile)))))
			}
			return out
		} else {
			log.Debug(" %v %v; contents:\n%v", ciEventFile, err, log.FormatJSON(string(lang.Continue(os.ReadFile(ciEventFile)))))
		}
	}
	return out
}

func envLoad(objects ...any) {
	for _, o := range objects {
		val := reflect.ValueOf(o)
		if val.Kind() != reflect.Ptr {
			panic(fmt.Errorf("expected pointer, got: %#v", o))
		} else {
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			panic(fmt.Errorf("expected pointer to struct, got: %#v", o))
		}
		envLoadVal(val)
	}
}

func envLoadVal(val reflect.Value) {
	switch val.Kind() {
	case reflect.Pointer, reflect.Interface:
		envLoadVal(val.Elem())
	case reflect.Struct:
	default:
		log.Trace("skipping env load for: %v", val)
		return
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		env := field.Tag.Get("env")
		if env == "" {
			continue
		}
		fieldVal := val.Field(i)
		switch fieldVal.Kind() {
		case reflect.String:
			fieldVal.SetString(config.Env(env, fieldVal.String()))
		case reflect.Bool:
			boolValue, _ := strconv.ParseBool(config.Env(env, strconv.FormatBool(fieldVal.Bool())))
			fieldVal.SetBool(boolValue)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			newVal := config.Env(env, "")
			if newVal != "" {
				newInt, err := strconv.ParseInt(newVal, 10, 64)
				if err != nil {
					log.Debug("unable to parse int value for: %v %v %v", env, newVal, err)
					continue
				}
				fieldVal.SetInt(newInt)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			newVal := config.Env(env, "")
			if newVal != "" {
				newUint, err := strconv.ParseUint(newVal, 10, 64)
				if err != nil {
					log.Debug("unable to parse Uint value for: %v %v %v", env, newVal, err)
					continue
				}
				fieldVal.SetUint(newUint)
			}
		case reflect.Float32, reflect.Float64:
			newVal := config.Env(env, "")
			if newVal != "" {
				newInt, err := strconv.ParseFloat(newVal, 64)
				if err != nil {
					log.Debug("unable to parse float value for: %v %v %v", env, newVal, err)
					continue
				}
				fieldVal.SetFloat(newInt)
			}
		case reflect.Complex64, reflect.Complex128:
			newVal := config.Env(env, "")
			if newVal != "" {
				newInt, err := strconv.ParseComplex(newVal, 64)
				if err != nil {
					log.Debug("unable to parse Complex value for: %v %v %v", env, newVal, err)
					continue
				}
				fieldVal.SetComplex(newInt)
			}
		case reflect.Pointer:
			envLoadVal(fieldVal)
		default:
			log.Trace("skipping env load for field: %v", field)
		}
	}
}
