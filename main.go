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

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func main() {
	// activate json logging
	log.SetFormatter(&log.JSONFormatter{})

	// consts
	const role = "edit"
	const separationString = "-"
	const generatedDefaulfSuffixLength = 5

	// create cmdline arguments
	prefix := flag.String("prefix", "tcn", "Optional: Prefix of namespace")
	namespace := flag.String("namespace", "", fmt.Sprintf(
		"Mandatory: main input parameter for the namespace name. "+
			"Notice that the full pattern of the (output) namespace is composed of the input parameters: "+
			"\n[<prefix>%s]<namespace>[%s<suffix>]",
		separationString, separationString))
	user := flag.String("user", "", "Optional: user that gets authorized in the created namespace")
	suffix := flag.String("suffix", "", fmt.Sprintf("Optional: Suffix of namespace. "+
		"If empty, a random string of %s characters will be appended", string(rune(generatedDefaulfSuffixLength))))
	mode := flag.String("mode", "create", "Mandatory: create|delete. Note, that create will first "+
		"delete the previous namespace as well!\n"+
		"delete: deletes namspaces matching '[<prefix>-]<namespace>', but not if the namespace already exists with\n"+
		"        the same suffix.\n"+
		"create: same as delete + creates a new one afterwards"+
		"        Note that there is a special case when no namespace will be created, but outputted to outFilePath:"+
		"        When you manually create a ns 'ab', and call tcn in create mode with params suffix=a, namespace=b."+
		"        There is one main reasons to this - passthrough:\n"+
		"        This way we can allow the user to specify a fixed namespace while still using the output of this "+
		"        task as his single source of namespace. Tekton has atm. no if/else logic in templating, so we "+
		"        can use the logic from within this task to allow pipeline users to toggle via config, whether "+
		"        to use a generated or a fixed namespace. \n"+
		"        To clarify further, tcn always produces a suffix (if you do not specify one yourself), therefore"+
		"        it can detect whether a ns exists that has the same ns (but without the suffix) - as in the case "+
		"        above")
	level := flag.String("level", "info", "Log level: panic|fatal|error|warn|info|debug|trace")
	outFilePath := flag.String("outFilePath", "", "If specified, will write the full output "+
		"namespace to this file path")

	// parse cmdline arguments
	flag.Parse()

	// init
	parsedLevel, err := log.ParseLevel(*level)
	if err != nil {
		panic(fmt.Sprintf("Could not parse log level from string: %s", *level))
	}
	log.SetLevel(parsedLevel)

	// validate
	if *mode != "create" && *mode != "delete" {
		log.Fatalf("Invalid mode '%s', must be create|delete", *mode)
	}
	namespaceNormalized, err := validateAndTransformToK8sName(*namespace, []rune(separationString)[0])
	if err != nil {
		log.Fatalf("Error parsing namespace name: %s", namespaceNormalized)
	}

	// prepare kubernetes client with in cluster configuration
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Could not read k8s cluster configuration: %s", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Could not create k8s client: %s", err)
	}

	// crafting namespace name
	ns := ""
	var prefixWithSeparator string
	if *prefix == "" {
		prefixWithSeparator = ""
	} else {
		prefixWithSeparator = *prefix + separationString
	}
	prefixAndNamespace := prefixWithSeparator + namespaceNormalized
	nsDraft := prefixAndNamespace + separationString + *suffix
	// generate randomstring for namespace postfix if buildhash is unset, avoiding collisions
	if *suffix == "" {
		randomstring := StringWithCharset(5, charset)
		ns = nsDraft + randomstring
	} else {
		ns = nsDraft
	}
	nsSpec := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	// get existing ns
	namespaceList, err := getNamespaceList(clientset)
	if err != nil {
		log.Errorf("Could not list namespace: %s", err)
	}
	if len(namespaceList.Items) == 0 {
		log.Warn("Namespace List is empty, seems fishy!")
	}

	// cleanup old ns
	cleanupNamespaces(clientset, prefixAndNamespace, ns, *namespaceList)

	// create new ns
	if !existsNamespace(namespaceList, *prefix) && *mode == "create" {
		log.Infof("Namespace with prefix '%s' does not exist (Note this does not mean that the namespace does "+
			"not exist with a suffix - that check comes later!)",
			*prefix)
		createNamespace(clientset, nsSpec, namespaceList)

		if *user != "" {
			log.Info("Assign role " + role + " in namespace " + ns + " to user " + *user)
			rb := &rbacv1.RoleBinding{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{Name: *namespace + "troubleshooter"},
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
	} else {
		if *mode == "create" {
			log.Infof("I did not touch k8s and not delete/create any namespace while in create mode. " +
				"Check the tcn (tekton create namespace) docs (-help) if you really desire to run in passthrough.")
			ns = *prefix
		}
	}

	// output
	if *outFilePath != "" {
		f, err := os.Create(*outFilePath)
		if err != nil {
			log.Fatalf("Error wriring file %s: %s", *outFilePath, err)
		}
		f.WriteString(ns)
	}
}

// replaces k8s invalid chars (separationRune) in inputString
func validateAndTransformToK8sName(inputString string, separationRune rune) (string, error) {
	// init
	var err error

	// pre-validate
	if inputString == "" {
		return "", errors.New("parameter namespace is required")
	}

	// lowercase
	inputStringLowerCase := strings.ToLower(inputString)

	// replace invalid characters
	r, _ := regexp.Compile("[-a-z\\d]")
	normalizedNameRunes := []rune("")
	for _, ch := range inputStringLowerCase {
		chs := string(ch)
		if !r.MatchString(chs) {
			log.Tracef("namespace '%s' contains invalid character: %s,"+
				"allowed are only ones that match the regex: %s, appending a '%s' instead of this character!",
				inputStringLowerCase, chs, r, string(separationRune))
			normalizedNameRunes = append(normalizedNameRunes, separationRune)
		} else {
			normalizedNameRunes = append(normalizedNameRunes, ch)
		}
	}

	// truncate too long name
	// RFC 1123 Label Names
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	if len(normalizedNameRunes) > 63 {
		normalizedNameRunes = normalizedNameRunes[:62]
	}

	// chomp minuses at beginning
	normalizedNameRunes = chompBeginningCharacter(normalizedNameRunes, separationRune)

	// chomp minuses at end
	normalizedNameRunes = chompEndingCharacter(normalizedNameRunes, separationRune)

	// convert rune array to string
	normalizedNameString := string(normalizedNameRunes)

	// post-validate
	if normalizedNameString == "" {
		return "",
			errors.New(fmt.Sprintf("namespace empty after matching all characters against regex: '%s'", r))
	}

	return normalizedNameString, err
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

func createRolebinding(clientset *kubernetes.Clientset,
	rb *rbacv1.RoleBinding,
	ns string) (*rbacv1.RoleBinding, error) {
	rb, err := clientset.RbacV1().RoleBindings(ns).Create(context.TODO(), rb, metav1.CreateOptions{})
	return rb, err
}

func createNamespace(
	clientset *kubernetes.Clientset,
	nsSpec *v1.Namespace,
	namespaceList *v1.NamespaceList,
) *v1.Namespace {
	log.Info("Considering to create namespace " + nsSpec.Name)
	if !existsNamespaceWithPrefix(namespaceList, nsSpec.Name) {
		ns, err := clientset.CoreV1().Namespaces().Create(context.TODO(), nsSpec, metav1.CreateOptions{})
		if err != nil {
			log.Fatalf("Error creating namespace %s, error was: %s", nsSpec, err)
		}
		log.Infof("Created Namespace %s", nsSpec.Name)
		return ns
	} else {
		log.Infof("Namespace matching %s already exists!", nsSpec.Name)
		return nsSpec
	}
}

func cleanupNamespaces(clientset *kubernetes.Clientset, pre string, ns string, nl v1.NamespaceList) {
	log.Infof("Considering to cleanup dangling namespaces with prefix: %s", pre)
	for _, n := range nl.Items {
		log.Tracef("Iterating over namespaces: current iteration: %s", n.Name)
		if strings.HasPrefix(n.Name, pre) && n.Name != ns {
			log.Infof("deleting namespace %s", n.Name)
			err := clientset.CoreV1().Namespaces().Delete(context.TODO(), n.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Fatalf("Error deleting namespace: %s", err)
			}
		}
	}
}

func getNamespaceList(clientset *kubernetes.Clientset) (*v1.NamespaceList, error) {
	nl, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	return nl, err
}

func StringWithCharset(length int, charset string) string {
	randombytes := make([]byte, length)
	for i := range randombytes {
		randombytes[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(randombytes)
}

func existsNamespaceWithPrefix(namespaceList *v1.NamespaceList, namespacePrefix string) bool {
	for _, ns := range namespaceList.Items {
		if strings.Contains(ns.Name, namespacePrefix) {
			return true
		}
	}
	return false
}

func existsNamespace(namespaceList *v1.NamespaceList, namespace string) bool {
	for _, ns := range namespaceList.Items {
		if ns.Name == namespace {
			return true
		}
	}
	return false
}
