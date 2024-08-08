package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/mikesmitty/edkey"
	"golang.org/x/crypto/ssh"
)

// generateEd25519Key generates a new Ed25519 private key and public key.
func generateEd25519Key() (string, string, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	publicKey, _ := ssh.NewPublicKey(pubKey)

	privateKey := pem.EncodeToMemory(&pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: edkey.MarshalED25519PrivateKey(privKey),
	})

	sPublicKey := string(ssh.MarshalAuthorizedKey(publicKey))

	return string(privateKey), sPublicKey, nil
}

// uploadToSSM uploads the given parameter values to AWS SSM with the specified parameter names.
func uploadToSSM(ssmParamPathBase, publicKey, privateKey, comment string) error {
	publicKeyName := filepath.Join(ssmParamPathBase, "public-key")
	privateKeyName := filepath.Join(ssmParamPathBase, "private-key")

	desc := fmt.Sprintf(
		"AWS EC2 SSH %%s (%s), for emergency use only. Generated %s",
		comment,
		time.Now().UTC().Format("2006-01-02"),
	)
	// Set up the session
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	ssmSvc := ssm.New(sess)

	// Put the public key parameter in SSM
	_, err := ssmSvc.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(publicKeyName),
		Description: aws.String(fmt.Sprintf(desc, "Public Key")),
		Type:        aws.String("SecureString"),
		Value:       aws.String(publicKey),
		Overwrite:   aws.Bool(true),
	})
	if err != nil {
		return err
	}

	// Put the private key parameter in SSM
	_, err = ssmSvc.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(privateKeyName),
		Description: aws.String(fmt.Sprintf(desc, "Private Key")),
		Type:        aws.String("SecureString"),
		Value:       aws.String(privateKey),
		Overwrite:   aws.Bool(true),
	})
	return err
}

func main() {
	// Define command line flags
	debug := flag.Bool("debug", false, "Enable debug mode")
	help := flag.Bool("h", false, "Display usage")
	comment := flag.String("comment", "ed25519 SSH Key", "Add a comment to the public key output")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-debug] [-comment <comment>] <parameter_name> \n", os.Args[0])
		flag.PrintDefaults()
	}

	// Parse command line flags
	flag.Parse()

	// Check if help flag is set
	if *help {
		flag.Usage()
		return
	}

	// Get the remaining positional arguments
	args := flag.Args()
	var paramPath string

	// Check if there is exactly one positional argument
	if len(args) == 1 {
		paramPath = args[0]
	} else {
		if *debug {
			paramPath = "/ec2/keypair/ed25519-ssh-private-key"
		} else {
			log.Fatal("Exactly one positional argument (parameterName) is required")
			flag.Usage()
			return
		}
	}

	// Generate the Ed25519 key
	privateKey, publicKey, err := generateEd25519Key()
	if err != nil {
		log.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	//Add comment to public key
	publicKey = strings.TrimSpace(publicKey) + " " + *comment

	if *debug {
		// Print the private key to the console in debug mode
		fmt.Printf("Private Key (PEM):\n%s\n", privateKey)
	} else {
		// Upload the private key to AWS SSM
		err = uploadToSSM(paramPath, publicKey, privateKey, *comment)
		if err != nil {
			log.Fatalf("Failed to upload private key to SSM: %v", err)
		} else {
			log.Printf("Successfully uploaded the private key to SSM")
		}
	}

	// Print the public key to the console
	fmt.Printf("Public Key: %s\n", publicKey)
	// Encode the public key to base64
	fmt.Println("Successfully processed the SSH key")
}
