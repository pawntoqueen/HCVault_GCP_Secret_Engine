package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "time"

    vault "github.com/hashicorp/vault/api"
)

func setEnv() error {
    // Set environment variables for Vault address and token
    if err := os.Setenv("VAULT_ADDR", "http://127.0.0.1:8200"); err != nil {
        log.Printf("Cannot set VAULT_ADDR")
        return err
    }
    if err := os.Setenv("VAULT_TOKEN", "hvs.R6hbTiWnHWDe9FuvTKgJ8Iew"); err != nil {
        log.Printf("Cannot set VAULT_TOKEN")
        return err
    }

    return nil
}

func createVaultClient() (*vault.Client, error) {
    // Configure Vault client with environment variables
    config := vault.DefaultConfig()
    config.Address = os.Getenv("VAULT_ADDR")

    // Create a new Vault client
    return vault.NewClient(config)
}

func createMountPoint(projectID string, vaultclient *vault.Client) error {
    // Create a mount point for the GCP secret engine
    path := "gcp/" + projectID
    mountInput := &vault.MountInput{
        Type: "gcp",
    }

    if err := vaultclient.Sys().Mount(path, mountInput); err != nil {
        log.Fatalf("Cannot create mount point: %v", err)
        return err
    }

    return nil
}

func writeConfig(projectID string, vaultclient *vault.Client) error {
    contents, err := ioutil.ReadFile("csm-cred.json")
    if err != nil {
        log.Fatalf("Cannot read GCP credentials: %v", err)
        return err
    }

    data := map[string]interface{}{"credentials": string(contents[:])}

    path := "gcp/" + projectID + "/config"
    _, err = vaultclient.Logical().Write(path, data)

    if err != nil {
        log.Fatalf("Cannot write GCP credentials: %v", err)
        return err
    }

    return nil
}

func writeRoleset(projectID string, vaultclient *vault.Client) error {
    var data map[string]interface{}

    bytes, err := ioutil.ReadFile("roleset.json")
    if err != nil {
        log.Fatalf("Cannot read roleset: %v", err)
        return err
    }

    // Unmarshal the JSON data into a map
    err = json.Unmarshal(bytes, &data)
    if err != nil {
        log.Fatalf("Cannot unmarshal roleset: %v", err)
        return err
    }

    path := "gcp/" + projectID + "/roleset/my-token-roleset"
    _, err = vaultclient.Logical().Write(path, data)

    if err != nil {
        log.Fatalf("Cannot write roleset: %v", err)
        return err
    }

    fmt.Printf("roleset data: ", data)
    return nil
}


func generateToken(projectID string, vaultclient *vault.Client) (string, error) {
	// Wait for Vault to generate a token for the roleset
	time.Sleep(8 * time.Second)

	// Read the generated token from Vault
	pathReq := "gcp/" + projectID + "/roleset/my-token-roleset/token"
	secret, err := vaultclient.Logical().Read(pathReq)
	if err != nil {
		log.Fatalf("Cannot read token: %v", err)
		return "", err
	}

	// Extract the token from the secret
	token := secret.Data["token"].(string)
	return token, nil
}

func stopVM(token string) error {
	// Define the request to stop the VM
	url := "https://compute.googleapis.com/compute/v1/projects/csm-pro/zones/us-central1-a/instances/test-csm-vm/stop"
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Add("Authorization", "Bearer "+token)

	// Send the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Cannot do request: %v", err)
		return err
	}
	defer res.Body.Close()

	// Print the response
	fmt.Println(res)

	return nil
}

func main() {
	// Initialize variables
	projectID := "222ddx6"

	// Set environment variables for Vault
	if err := setEnv(); err != nil {
		log.Fatalf("Cannot set environment variables: %v", err)
	}

	// Create a new Vault client
	vaultclient, err := vault.NewClient(vault.DefaultConfig())
	if err != nil {
		log.Fatalf("Cannot create Vault client: %v", err)
	}
	defer vaultclient.ClearToken()

	// Create a mount point for the GCP secret engine
	path := "gcp/" + projectID
	mountInput := &vault.MountInput{
		Type: "gcp",
	}
	if err := vaultclient.Sys().Mount(path, mountInput); err != nil {
		log.Fatalf("projectID already exists: %v", err)
	}

	// Write GCP credentials to Vault
	if err := writeConfig(projectID, vaultclient); err != nil {
		log.Fatalf("Cannot write GCP credentials: %v", err)
	}

	// Write roleset data to Vault
	if err := writeRoleset(projectID, vaultclient); err != nil {
		log.Fatalf("Cannot write roleset: %v", err)
	}

	// Generate a token for the roleset
	token, err := generateToken(projectID, vaultclient)
	if err != nil {
		log.Fatalf("Cannot generate token: %v", err)
	}

	// Stop the VM
	if err := stopVM(token); err != nil {
		log.Fatalf("Cannot stop VM: %v", err)
	}
}
