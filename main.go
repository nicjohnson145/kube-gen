package main

import (
	"context"
	"encoding/base64"
	"flag"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path"
	"text/template"
)

var (
	name       string
	namespace  string
	kubeconfig string
	outfile    string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig")
	flag.StringVar(&name, "name", "", "Service account name")
	flag.StringVar(&namespace, "namespace", "default", "Namespace of service account")
	flag.StringVar(&outfile, "out", "", "Output to specified file instead of stdout")
}

func setupClient() (*kubernetes.Clientset, *restclient.Config) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Setup standard client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err.Error())
	}

	return clientset, config
}

func handleCommandLine() {
	if name == "" {
		log.Fatal("-name must be given")
	}

	if kubeconfig == "" {
		dir, err := os.UserHomeDir()
		check(err)
		kubeconfig = path.Join(dir, ".kube", "config")
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type Output struct {
	CA             string
	Server         string
	ServiceAccount string
	Namespace      string
	Token          string
}

var tmplS string = `
apiVersion: v1
kind: Config
clusters:
  - name: kubernetes
    cluster:
      certificate-authority-data: {{.CA}}
      server: {{.Server}}
contexts:
  - name: {{.ServiceAccount}}@kubernetes
    context:
      cluster: kubernetes
      namespace: {{.Namespace}}
      user: {{.ServiceAccount}}
users:
  - name: {{.ServiceAccount}}
    user:
      token: {{.Token}}
current-context: {{.ServiceAccount}}@kubernetes
`

func main() {
	flag.Parse()
	ctx := context.TODO()
	client, config := setupClient()

	sa, err := client.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	check(err)

	secretName := sa.Secrets[0].Name

	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	check(err)

	tmpl := template.Must(template.New("thing").Parse(tmplS))
	data := Output{
		CA:             string(base64.StdEncoding.EncodeToString(secret.Data["ca.crt"])),
		Server:         config.Host,
		ServiceAccount: name,
		Namespace:      namespace,
		Token:          string(secret.Data["token"]),
	}

	out := os.Stdout
	if outfile != "" {
		f, err := os.Create(outfile)
		check(err)
		defer f.Close()

		out = f
	}

	err = tmpl.Execute(out, data)
	check(err)
}
