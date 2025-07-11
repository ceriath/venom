package kubernetes

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/ovh/venom"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Name of executor
const Name = "kubernetes"

// New returns a new Executor
func New() venom.Executor {
	return &Executor{}
}

// Headers represents header HTTP for Request
type Headers map[string]string

// Executor struct. Json and yaml descriptor are used for json output
type Executor struct {
	Method         string `json:"method" yaml:"method"`
	Namespace      string `json:"namespace" yaml:"namespace"`
	Resource       string `json:"resource" yaml:"resource"`
	EntryName      string `json:"entryname" yaml:"entryname"`
	Data           string `json:"data" yaml:"data"`
	ConfigFilePath string `json:"configfilepath" yaml:"configfilepath"`
	LabelSelector  string `json:"labelselector" yaml:"labelselector"`
}

// Result represents a step result
type Result struct {
	Code      int         `json:"code,omitempty" yaml:"code,omitempty"`
	BodyJSON  interface{} `json:"bodyjson,omitempty" yaml:"bodyjson,omitempty"`
	Systemerr string      `json:"systemerr,omitempty" yaml:"systemerr,omitempty"` // put in testcase.Systemerr by venom if present
}

// Run executes TestStep
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	// transform step to Executor Instance
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}
	// if no config file path is provided, use user default config
	// use the current context in kubeconfig
	// create the clientset
	clientset := buildClient(e)
	var bytes_s1 []byte
	var resultCode *int = new(int)
	switch e.Method {
	case "version":
		res, err := clientset.ServerVersion()
		if err != nil {
			return nil, err
		}
		bytes_s1, err = json.Marshal(res)
	case "get":
		var res runtime.Object
		var err error
		req := clientset.CoreV1().RESTClient().Get().Namespace(e.Namespace).Resource(e.Resource)
		if e.EntryName != "" {
			req = req.Name(e.EntryName)
		}
		if e.LabelSelector != "" {
			req = req.Param("labelSelector", e.LabelSelector)
		}

		res, err = req.Do(ctx).StatusCode(resultCode).Get()

		if err != nil && *resultCode != 404 {
			return nil, err
		}
		bytes_s1, err = json.Marshal(res)
	case "delete":
		var res runtime.Object
		var err error
		if e.EntryName != "" {
			res, err = clientset.CoreV1().RESTClient().Delete().Namespace(e.Namespace).Resource(e.Resource).Name(e.EntryName).Do(ctx).StatusCode(resultCode).Get()
		} else {
			return nil, fmt.Errorf("Valid Name is required: %s", err)
		}
		if err != nil {
			return nil, err
		}
		bytes_s1, err = json.Marshal(res)
	case "create":
		var res runtime.Object
		var err error
		res, err = clientset.CoreV1().RESTClient().Post().Namespace(e.Namespace).Resource(e.Resource).Body([]byte(e.Data)).Do(ctx).StatusCode(resultCode).Get()
		if err != nil {
			return nil, err
		}
		bytes_s1, err = json.Marshal(res)
	default:
		return nil, fmt.Errorf("Method not supported: " + e.Method)
	}

	// decode json output
	var m interface{}
	decoder := json.NewDecoder(strings.NewReader(string(bytes_s1)))
	decoder.UseNumber()
	outputCode := *resultCode
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("Result not valid JSON: %s", err)
	}

	// prepare result
	r := Result{
		Code:     outputCode, // return Output Code
		BodyJSON: m,          // return Output string
	}

	return r, nil
}

func buildClient(e Executor) *kubernetes.Clientset {
	var kubeconfig *string
	if e.ConfigFilePath != "" {
		kubeconfig = &e.ConfigFilePath
	} else {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()
	}

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}
