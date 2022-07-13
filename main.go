package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"regexp"
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
	role := "edit"

	//Create and parse cmdline arguments
	branch := flag.String("branchname", "", "Mandatory: input parameter for the branche name")
	user := flag.String("user", "", "Optional: the value is authorized as a user in the created namespace")
	hash := flag.String("buildhash", "", "Optional: input parameter for the build hash")
	flag.Parse()

	//Need branchname to map tekton pipeline with namespace
	branchNormalized, err := validateAndNormalizeBranch(*branch)
	if err != nil {
		log.Fatalf("Error parsing branch name: %s", branchNormalized)
	}

	//prepare kubernetes in cluster configuration
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	//Crafting namespace name
	ns := ""
	prefix := pre + "-" + branchNormalized + "-" + *hash
	//Generate randomstring for namespace postfix if buildhash is unset, avoiding collisions
	if *hash == "" {
		randomstring := StringWithCharset(5, charset)
		ns = prefix + randomstring
	} else {
		ns = prefix
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
		log.Info("Created namespace " + ns)
	}

	log.Info("Start to create namespace " + ns)
	if *user != "" {
		log.Info("Assign role " + role + " in namespace " + ns + " to user " + *user)
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
				Name:     role,
			},
		}
		_, err = createRolebinding(clientset, rb, nsSpec.GetObjectMeta().GetName())
		if err != nil {
			log.Error(err)
		} else {
			log.Info("Created rolebinding " + rb.Name + " in namespace " + ns)
		}
	} else {
		log.Info("No user was defined - skipping role assignment")
	}

}

// removes k8s invalid chars
func validateAndNormalizeBranch(branch string) (string, error) {
	// init
	var err error
	const separationRune = '-'

	// pre-validated
	if branch == "" {
		return "", errors.New("parameter branchname is required")
	}

	// lowercase
	branchLowerCase := strings.ToLower(branch)

	// replace invalid characters
	r, _ := regexp.Compile("[-a-z\\d]")
	normalizedBranchRunes := []rune("")
	for _, ch := range branchLowerCase {
		chs := string(ch)
		if !r.MatchString(chs) {
			log.Tracef("branchname '%s' contains invalid character: %s,"+
				"allowed are only ones that match the regex: %s, appending a minus(-) instead of this character!",
				branchLowerCase, chs, r)
			normalizedBranchRunes = append(normalizedBranchRunes, separationRune)
		} else {
			normalizedBranchRunes = append(normalizedBranchRunes, ch)
		}
	}

	// chomp minuses at beginning
	normalizedBranchRunes = chompBeginningCharacter(normalizedBranchRunes, separationRune)

	// chomp minuses at end
	normalizedBranchRunes = chompEndingCharacter(normalizedBranchRunes, separationRune)

	// post-validated
	normalizedBranchString := string(normalizedBranchRunes)
	if normalizedBranchString == "" {
		return "", errors.New(
			fmt.Sprintf("branchname empty after matching all characters against regex: '%s'", r))
	}

	return normalizedBranchString, err
}

func chompBeginningCharacter(runearr []rune, runechar rune) []rune {
	chomping := true
	var chompedRune []rune
	for _, cr := range runearr {
		if chomping && cr == runechar {
			log.Tracef("chomping character %s from string %s", string(cr), string(runechar))
		} else {
			chompedRune = append(chompedRune, cr)
			chomping = false
		}
	}
	return chompedRune
}

func chompEndingCharacter(runearr []rune, runechar rune) []rune {
	if len(runearr) == 0 {
		return []rune{}
	}
	if runearr[len(runearr)-1] == runechar {
		return chompEndingCharacter(runearr[:len(runearr)-1], runechar)
	} else {
		return runearr
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
