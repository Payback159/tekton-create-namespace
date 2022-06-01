package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()),
)

func main() {
	//activate json logging
	log.SetFormatter(&log.JSONFormatter{})
	//Prefix for the namespaces
	pre := "tcn"

	//Create and parse cmdline arguments
	branch := flag.String("branchname", "", "input parameter for the branche name")
	hash := flag.String("buildhash", "", "input parameter for the build hash")
	user := flag.String("user", "", "the value is authorized as a user in the created namespace")
	role := flag.String("role", "edit", "by default the edit role is used, with this flag a CustomeRole can be referenced")
	flag.Parse()

	//prepare kubernetes in cluster configuration
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	//Generate randomstring for namespace postfix, avoiding collisions
	randomstring := StringWithCharset(5, charset)
	prefix := pre + "-" + *branch + "-" + *hash
	ns := prefix + "-" + randomstring
	log.Info("Start to create namespace " + ns)
	if *user != "" {
		log.Info("Assign role " + *role + " in namespace " + ns + " to user " + *user)
	} else {
		log.Info("No user was defined - skipping role assignment")
	}

	nsSpec := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	//run cleanupNamespaces in goroutine
	go cleanupNamespaces(clientset, prefix, ns)

	_, err = createNamespace(clientset, nsSpec)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Info("Created namespace %s", ns)
	}

	if *user != "" {
		rb := &rbacv1.RoleBinding{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{Name: pre + "troubleshooter"},
			Subjects: []rbacv1.Subject{
				{
					APIGroup: rbacv1.GroupName,
					Kind:     rbacv1.UserKind,
					Name:     *user,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     *role,
			},
		}
		_, err = createRolebinding(clientset, rb, nsSpec.GetObjectMeta().GetName())
		if err != nil {
			log.Error(err)
		} else {
			log.Info("Created rolebinding %s in namespace %s", rb.Name, ns)
		}
	}

}

func createRolebinding(clientset *kubernetes.Clientset, rb *rbacv1.RoleBinding, ns string) (*rbacv1.RoleBinding, error) {
	rb, err := clientset.RbacV1().RoleBindings(ns).Create(context.TODO(), rb, metav1.CreateOptions{})
	return rb, err
}

func createNamespace(clientset *kubernetes.Clientset, nsSpec *v1.Namespace) (*v1.Namespace, error) {
	ns, err := clientset.CoreV1().Namespaces().Create(context.TODO(), nsSpec, metav1.CreateOptions{})
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	return ns, err
}

func cleanupNamespaces(clientset *kubernetes.Clientset, pre string, ns string) {
	log.Info("Starting to cleanup dangling namespaces")
	nl, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
	}
	for _, n := range nl.Items {
		if strings.HasPrefix(n.Name, pre) && n.Name != ns {
			err = clientset.CoreV1().Namespaces().Delete(context.TODO(), n.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Error(err)
			}
		}
	}
}

func StringWithCharset(length int, charset string) string {

	randombytes := make([]byte, length)
	for i := range randombytes {
		randombytes[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(randombytes)
}
