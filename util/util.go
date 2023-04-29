package util

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mohamed-rafraf/k8s-auth-kube/config"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func createSA(saName, namespace string) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
		},
	}
	_, err := config.Clientset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil

}

func createToken(saName, namespace string, minutes int) (string, error) {
	expirationSeconds := int64(minutes * 60)

	tokenRequest := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}

	token, err := config.Clientset.CoreV1().ServiceAccounts(namespace).CreateToken(context.Background(), saName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return token.Status.Token, nil

}

func DeleteRole(name, namespace string) error {
	err := config.Clientset.RbacV1().Roles(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// DeleteRoleBinding deletes a role binding with the given name and namespace
func DeleteRoleBinding(name, namespace string) error {
	err := config.Clientset.RbacV1().RoleBindings(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func createRoleBinding(roleName string, serviceAccountName string, namespace string) error {

	roleRef := rbacv1.RoleRef{
		Kind:     "Role",
		Name:     roleName,
		APIGroup: "rbac.authorization.k8s.io",
	}

	subject := rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      serviceAccountName,
		Namespace: "k8s-auth",
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef:  roleRef,
	}

	_, err := config.Clientset.RbacV1().RoleBindings(namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func GenerateToken(message string) (string, error) {
	// Split the message
	parts := strings.Split(string(message), ",")
	if len(parts) != 3 {
		return "", errors.New("wrong message format")
	}
	// Get the user
	user := strings.Split(parts[1], "=")[1]

	// get the roles
	rbac, _ := (base64.StdEncoding.DecodeString(strings.Split(parts[2], "=")[1]))

	// create a service account for that user
	user = strings.ReplaceAll(user, "@", "-at-")

	err := DeleteUserFromCSV(user)

	if err != nil {
		return "", err
	}

	err = DeleteRoleBindingsBelongsToUser(user)

	if err != nil {
		return "", err
	}

	err = DeleteRolesBelongsToUser(user)

	if err != nil {
		return "", err
	}

	if err != nil {
		return "", err
	}
	err = DeleteServiceAccount(user, "k8s-auth")
	err = createSA(user, "k8s-auth")

	if err != nil {
		return "", err
	}

	// Split the content into separate roles resources.
	resources := strings.Split(string(rbac), "---\n")

	// Generate a decoder that parse the data to roles objects
	decode := scheme.Codecs.UniversalDeserializer().Decode

	// Loop through each RBAC resource and apply it.
	for i, resource := range resources {

		// Create a new YAML serializer.
		obj, _, err := decode([]byte(resource), nil, nil)
		if err != nil {
			return "", err
		}

		// Parse the object to role object
		role := obj.(*rbacv1.Role)
		if err != nil {
			return "", err
		}

		// rename the role
		role.Name = user + "-" + strconv.Itoa(i)

		// Create the new role
		_, err = config.Clientset.RbacV1().Roles(role.Namespace).Create(context.TODO(), role, metav1.CreateOptions{})
		if err != nil {
			return "", err
		}

		// Create The New RoleBinding
		err = createRoleBinding(role.Name, user, role.Namespace)
		if err != nil {
			return "", err
		}
	}

	err = createSecret(user)
	if err != nil {
		return "", err
	}

	time.Sleep(50 * time.Millisecond)

	secret, err := config.Clientset.CoreV1().Secrets("k8s-auth").Get(context.Background(), user, metav1.GetOptions{})

	if err != nil {
		return "", err
	}
	token, _ := secret.Data["token"]

	// Calculate the termination time based on the duration
	terminationTime := time.Now().Add(time.Minute * time.Duration(10))
	AddUserToCSV(user, terminationTime)

	log.Println(user, "is authenticated")

	return string(token), nil
}

func createSecret(name string) error {
	// Create the secret object
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": name,
			},
		},
		Type: "kubernetes.io/service-account-token",
	}

	_, err := config.Clientset.CoreV1().Secrets("k8s-auth").Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func DeleteRolesBelongsToUser(user string) error {

	roles, err := config.Clientset.RbacV1().Roles("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, role := range roles.Items {
		if strings.Contains(role.Name, user) {
			err := config.Clientset.RbacV1().Roles(role.Namespace).Delete(context.Background(), role.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteRoleBindingsBelongsToUser(user string) error {

	roleBindings, err := config.Clientset.RbacV1().RoleBindings("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, roleBinding := range roleBindings.Items {
		if strings.Contains(roleBinding.Name, user) {
			err := config.Clientset.RbacV1().RoleBindings(roleBinding.Namespace).Delete(context.Background(), roleBinding.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func DeleteServiceAccount(namespace, name string) error {
	err := config.Clientset.CoreV1().ServiceAccounts(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func AddUserToCSV(user string, terminationTime time.Time) error {
	// Open CSV file
	file, err := os.OpenFile("users.csv", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)

	// Write record to CSV file
	record := []string{user, terminationTime.Format(time.RFC3339)}
	err = writer.Write(record)
	if err != nil {
		return err
	}

	// Flush CSV writer
	writer.Flush()

	return nil
}

func DeleteUserFromCSV(userID string) error {
	// Open CSV file
	file, err := os.OpenFile("users.csv", os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create CSV reader
	reader := csv.NewReader(file)

	// Read all records from CSV file
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	// Find the record to delete
	var indexToDelete int = -1
	for i, record := range records {
		if record[0] == userID {
			indexToDelete = i
			break
		}
	}

	// If the record was found, delete it
	if indexToDelete >= 0 {
		records = append(records[:indexToDelete], records[indexToDelete+1:]...)
	} else {
		// Close CSV file
		file.Close()
		return nil
	}

	// Close CSV file
	file.Close()

	// Reopen CSV file in write mode
	file, err = os.OpenFile("users.csv", os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)

	// Write all remaining records to CSV file
	err = writer.WriteAll(records)
	if err != nil {
		return err
	}

	// Flush CSV writer
	writer.Flush()

	return nil
}

func DeleteRowByTerminationTime() error {
	// Open CSV file
	file, err := os.OpenFile("users.csv", os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create CSV reader
	reader := csv.NewReader(file)

	// Read all records from CSV file
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	// Check if the termination time of any record has passed
	for _, record := range records {
		terminationTime, err := time.Parse(time.RFC3339, record[1])
		if err != nil {
			return err
		}

		if terminationTime.Before(time.Now()) {
			// Delete the row from the CSV file
			err = DeleteUserFromCSV(record[0])
			if err != nil {
				return err
			}

			err = DeleteRoleBindingsBelongsToUser(record[0])

			if err != nil {
				return err
			}

			err = DeleteRolesBelongsToUser(record[0])

			if err != nil {
				return err
			}
			err = DeleteServiceAccount("k8s-auth", record[0])

			if err != nil {
				return err
			}
			// Print a message to indicate that the row has been deleted
			log.Println(record[0], "logout")
			return nil
		}
	}

	return nil
}

func ClearToken(message string) error {
	// Split the message
	parts := strings.Split(string(message), ",")
	if len(parts) != 2 {
		return errors.New("Wrong Message Format")
	}
	// Get the user
	user := strings.Split(parts[1], "=")[1]

	// create a service account for that user
	user = strings.ReplaceAll(user, "@", "-at-")

	if !userExists(user) {
		return nil
	}
	// Delete the row from the CSV file
	err := DeleteUserFromCSV(user)
	if err != nil {
		return err
	}

	err = DeleteRoleBindingsBelongsToUser(user)

	if err != nil {
		return err
	}

	err = DeleteRolesBelongsToUser(user)

	if err != nil {
		return err
	}
	err = DeleteServiceAccount("k8s-auth", user)

	if err != nil {
		return err
	}
	log.Println(user, "is deleted from the cluster!")
	// Print a message to indicate that the row has been deleted
	return nil
}

func userExists(userID string) bool {
	// Open CSV file
	file, err := os.Open("users.csv")
	if err != nil {
		return false
	}
	defer file.Close()

	// Create CSV reader
	reader := csv.NewReader(file)

	// Read all records from CSV file
	records, err := reader.ReadAll()
	if err != nil {
		return false
	}

	// Check if the userID exists in any record
	for _, record := range records {
		if record[0] == userID {
			return true
		}

	}

	return false
}
