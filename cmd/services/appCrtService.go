package services

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

//go:embed appCnf.tmpl
var applicationCnf []byte

// outputDir string, intermediateCaCnf string, intermediateCaCrt string, intermediateCaKey string, rootCaCrt string, appName string, commonName string, altNames []string, p12 bool
type CreateAppCrtOptions struct {
	// OutputDir is the output directory for created certificates
	OutputDir string
	// IntermediateCaCnf is the intermediate ca cnf file
	IntermediateCACnf string
	// IntermediateCaCrt is the intermediate ca crt file
	IntermediateCACrt string
	// IntermediateCaKey is the intermediate ca key file
	IntermediateCAKey string
	// RootCaCrt is the root ca crt file
	RootCACrt string
	// AppName is the name of the application
	AppName string
	// CommonName is the common name of the application
	CommonName string
	// AltNames is the alternative names of the application
	AltNames []string
	// P12 is the flag for creating p12 files
	P12 bool
}

func CreateAppCrt(opts CreateAppCrtOptions) {
	// Create app directory if not exists:
	appCrtDir := fmt.Sprintf("%s/%s", opts.OutputDir, opts.AppName)
	if _, err := os.Stat(appCrtDir); os.IsNotExist(err) {
		log.Debug("App dir is being created", appCrtDir)
		err := os.Mkdir(appCrtDir, 0700)
		if err != nil {
			log.Fatal("Error while creating App dir: ", err)
		}
		log.Debug("App dir generated at ", appCrtDir)
	} else {
		log.Debug("App dir already exists, skipping.")
	}

	// Create app key with openssl
	applicationKeyFile := fmt.Sprintf("%s/%s.key", appCrtDir, opts.AppName)
	if _, err := os.Stat(applicationKeyFile); os.IsNotExist(err) {
		log.Debug("App Key is being created.")
		createAppKeyCmd := exec.Command("openssl", "genpkey", "-algorithm", "RSA", "-out", applicationKeyFile)
		err = createAppKeyCmd.Run()
		if err != nil {
			log.Fatal("Error while creating App Key: ", err)
		}
		log.Debug("App Key generated at ", applicationKeyFile)
	} else {
		log.Debug("App Key already exists, skipping.")
	}

	// Create app cnf file
	applicationCnfFile := fmt.Sprintf("%s/%s.cnf", appCrtDir, opts.AppName)
	if _, err := os.Stat(applicationCnfFile); os.IsNotExist(err) {
		log.Debug("App Cnf being created.")

		appCnf, err := prepareAppCnf(opts.AppName, opts.CommonName, opts.AltNames)
		if err != nil {
			log.Fatal("Error while creating App Cnf from template:", err)
			return
		}

		err = os.WriteFile(applicationCnfFile, appCnf, os.ModePerm)
		if err != nil {
			log.Fatal("Error while writing App Cnf to file: ", err)
		}
		log.Debug("App Cnf generated at ", applicationCnfFile)
	} else {
		log.Debug("App Cnf already exists, skipping.")
	}

	// Create default CA App csr file
	applicationCsrFile := fmt.Sprintf("%s/%s.csr", appCrtDir, opts.AppName)
	if _, err := os.Stat(applicationCsrFile); os.IsNotExist(err) {
		log.Debug("App Crt being created")
		createAppCsrCmd := exec.Command(
			"openssl", "req", "-new",
			"-key", applicationKeyFile,
			"-config", applicationCnfFile,
			"-out", applicationCsrFile,
		)
		err = createAppCsrCmd.Run()
		if err != nil {
			log.Fatal("Error while creating App Csr: ", err)
		}
		log.Debug("App Csr generated at ", applicationCsrFile)
	} else {
		log.Debug("App Csr already exists, skipping.")
	}

	// Create default CA intermediate CA crt file
	applicationCrtFile := fmt.Sprintf("%s/%s.crt", appCrtDir, opts.AppName)
	if _, err := os.Stat(applicationCrtFile); os.IsNotExist(err) {
		log.Debug("App Crt being created.")
		createAppCrtCmd := exec.Command(
			"openssl", "x509", "-req",
			"-in", applicationCsrFile,
			"-CA", opts.IntermediateCACrt,
			"-CAkey", opts.IntermediateCAKey,
			"-CAcreateserial",
			"-days", "365",
			"-extensions", "v3_ext",
			"-extfile", applicationCnfFile,
			"-out", applicationCrtFile,
		)
		err = createAppCrtCmd.Run()
		if err != nil {
			log.Fatal("Error while creating App Crt: ", err)
		}
		log.Debug("App Crt generated at ", applicationCrtFile)
	} else {
		log.Debug("App Crt already exists, skipping.")
	}

	// Create fullchain cert file.
	rootCaCrtContent, err := os.ReadFile(opts.RootCACrt)
	if err != nil {
		log.Fatal("Error while reading Root CA cert for fullchain:", err)
		return
	}

	intermediateCaCrtContent, err := os.ReadFile(opts.IntermediateCACrt)
	if err != nil {
		log.Fatal("Error while reading Intermediate CA cert for fullchain:", err)
		return
	}

	appCrtContent, err := os.ReadFile(applicationCrtFile)
	if err != nil {
		log.Fatal("Error while reading App cert for fullchain:", err)
		return
	}

	appFullchainCrtFile := appCrtDir + "/fullchain.crt"
	if _, err := os.Stat(appFullchainCrtFile); os.IsNotExist(err) {
		log.Debug("App fullchain crt being created.")
		file, err := os.Create(appFullchainCrtFile)
		if err != nil {
			log.Fatal("Error while creating fullchain crt file:", err)
		}
		defer file.Close()

		fullchainCrtFile, err := os.OpenFile(appFullchainCrtFile, os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			log.Fatal("Error while opening fullchain crt file:", err)
		}
		defer fullchainCrtFile.Close()

		_, err = fullchainCrtFile.Write(appCrtContent)
		if err != nil {
			log.Fatal("Error while writing the app crt to the fullchain crt file:", err)
		}

		_, err = fullchainCrtFile.Write(intermediateCaCrtContent)
		if err != nil {
			log.Fatal("Error while writing the intermediate ca cert to file fullchain file:", err)
		}

		_, err = fullchainCrtFile.Write(rootCaCrtContent)
		if err != nil {
			log.Fatal("Error while writing the root ca crt to the fullchain crt file:", err)
		}
		log.Debug("App Fullchain crt generated at ", applicationCrtFile)
	} else {
		log.Debug("App fullchain crt already exits, skipping.")
	}

	applicationPfxFile := fmt.Sprintf("%s/%s.pfx", appCrtDir, opts.AppName)
	if _, err := os.Stat(applicationPfxFile); os.IsNotExist(err) {
		if opts.P12 {
			log.Debug("Creating p12 files.")
			createAppCrtCmd := exec.Command(
				"openssl", "pkcs12",
				"-in", appFullchainCrtFile,
				"-inkey", applicationKeyFile,
				"-password", "pass:changeit",
				"-export",
				"-out", applicationPfxFile,
			)
			createAppCrtCmd.Dir = appCrtDir
			err = createAppCrtCmd.Run()
			if err != nil {
				log.Fatal("Error while creating App Crt: ", err)
			}
			log.Debug("App Pfx generated at ", applicationCrtFile)
		}
	} else {
		log.Debug("App pfx already exits, skipping.")
	}
	log.Info("App certs created successfully.")
	log.Info("App name: ", opts.AppName)
	log.Info("Domains: ", opts.AltNames)
	log.Info("To see your cert files, please check the dir: ", appCrtDir)
	if os.Getenv("CONTAINER") == "true" {
		log.Warn("You are running the crtforge from container.")
		log.Info("The paths you see in the logs is not valid.")
		log.Info("You should replace /root with your own home directory.")
		log.Info("For example /root/.config/crtforge /home/user/.config/crtforge")
	}
}

func prepareAppCnf(appName string, commonName string, altNames []string) ([]byte, error) {
	tmpl, err := template.New("applicationCnf").Parse(string(applicationCnf))
	if err != nil {
		return nil, err
	}
	vars := make(map[string]interface{})
	vars["appName"] = appName
	vars["commonName"] = commonName
	vars["altNames"] = generateAltNames(altNames)

	var output bytes.Buffer
	if err := tmpl.Execute(&output, vars); err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func generateAltNames(altNames []string) string {
	var dnsLines []string
	for i, altName := range altNames {
		dnsLines = append(dnsLines, fmt.Sprintf("DNS.%d = %s", i+1, altName))
	}
	return strings.Join(dnsLines, "\n")
}
