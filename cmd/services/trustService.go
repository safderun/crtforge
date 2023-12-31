package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func TrustCrt(crtPath string) {
	os := detectOs()
	fmt.Println("Os:", os)
	if isLinux() {
		trustCrtOnLinux(&crtPath)
	} else if isMacos() {
		err := trustCrtOnMacos(&crtPath)
		if err != nil {
			log.Fatal("Error while trusting cert: ", err)
		}
	} else {
		fmt.Println("Unknown OS. Can not trust the cert.")
	}
}

func isLinux() bool {
	return detectOs() == "linux"
}

func isMacos() bool {
	return detectOs() == "darwin"
}

func detectOs() string {
	return runtime.GOOS
}

func trustCrtOnLinux(crtPath *string) error {
	fmt.Println(*crtPath, "is being trusted on Linux...")

	// sudoPermission := hasSudoPermissions()
	// if !sudoPermission {
	// 	return fmt.Errorf("you don't have sudo permissions to add cert to keychain")
	// }
	// fmt.Println("Sudo permission: ", sudoPermission)

	crtPathSplitted := strings.Split(*crtPath, "/")
	for i, j := 0, len(crtPathSplitted)-1; i < j; i, j = i+1, j-1 {
		crtPathSplitted[i], crtPathSplitted[j] = crtPathSplitted[j], crtPathSplitted[i]
	}

	fmt.Println(crtPathSplitted)
	caName := crtPathSplitted[3] + "-" + crtPathSplitted[2] + "-" + crtPathSplitted[0]
	fmt.Println(caName)

	var caPath string
	if _, err := os.Stat("/etc/debian_version"); !os.IsNotExist(err) {
		caPath = "/usr/local/share/ca-certificates/" + caName // Debian/Ubuntu
	} else if _, err := os.Stat("/etc/arch-release"); !os.IsNotExist(err) {
		caPath = "/etc/ca-certificates/trust-source/anchors/" + caName // Arch
	} else if _, err := os.Stat("/etc/redhat-release"); !os.IsNotExist(err) {
		caPath = "/etc/pki/ca-trust/source/anchors/" + caName // RedHat/CentOS
	} else if _, err := os.Stat("/etc/fedora-release"); !os.IsNotExist(err) {
		caPath = "/etc/pki/ca-trust/source/anchors/" + caName // Fedora
	} else {
		// Default to Debian path
		caPath = "/usr/local/share/ca-certificates/" + caName
	}

	cpCmd := exec.Command("sudo", "cp", *crtPath, caPath)
	err := cpCmd.Run()
	if err != nil {
		return fmt.Errorf("error while adding cert to system")
	}

	return nil
}

func trustCrtOnMacos(crtPath *string) error {
	fmt.Println(*crtPath, "is being trusted on MacOS...")

	isCrtTrustedCommand := exec.Command("security", "verify-cert", "-c", *crtPath)
	err := isCrtTrustedCommand.Run()
	if err == nil {
		fmt.Println(*crtPath, "is already found on keychain")
		return nil
	}
	fmt.Println(*crtPath, "will be trusted on keychain")

	allowCommand := exec.Command("sudo", "security", "authorizationdb", "write", "com.apple.trust-settings.admin", "allow")
	output, err := allowCommand.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed while getting permission to add cert to the keychain: %s", output)
	}

	trustCrtCommand := exec.Command("sudo", "security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", *crtPath)
	output, err = trustCrtCommand.CombinedOutput()
	if err != nil {
		fmt.Println("Command output:", string(output))
		return fmt.Errorf("failed to add cert to keychain: %w", err)
	}

	disallowCommand := exec.Command("sudo", "security", "authorizationdb", "remove", "com.apple.trust-settings.admin")
	output, err = disallowCommand.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed while removing keychain permission : %s", output)
	}

	fmt.Println(*crtPath, "has been added to keychain successfully.")
	return nil

}
