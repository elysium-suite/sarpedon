package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func validateUpdate(plainUpdate string) (map[string]string, error) {
	updateMap := make(map[string]string)
	splitUpdate := strings.Split(plainUpdate, "|-SP-|")
	if len(splitUpdate) != 13 {
		return updateMap, errors.New("Error splitting input.")
	}
	for i := 0; i < len(splitUpdate)-2; i += 2 {
		fmt.Println("MAPPING", splitUpdate[i], "to", splitUpdate[i+1])
		updateMap[splitUpdate[i]] = splitUpdate[i+1]
	}

	fields := []string{"team", "image", "score", "challenge", "vulns", "time"}
	for _, f := range fields {
		if !validateString(updateMap[f]) {
			return updateMap, errors.New("Field " + f + " contained invalid characters")
		}
	}

	if !validateTeam(updateMap["team"]) {
		return updateMap, errors.New("Invalid team: " + updateMap["team"])
	}

	if !validateImage(updateMap["image"]) {
		return updateMap, errors.New("Invalid image: " + updateMap["image"])
	}

	return updateMap, nil
}

func validateTeam(teamName string) bool {
	for _, team := range sarpConfig.Team {
		if team.Id == teamName {
			return true
		}
		if team.Alias == teamName {
			return true
		}
	}
	return false
}

func validateImage(imageName string) bool {
	for _, image := range sarpConfig.Image {
		if image.Name == imageName {
			return true
		}
	}
	return false
}

func parseVulns(vulnText string) vulnWrapper {
	wrapper := vulnWrapper{}
	vulnText, err := hexDecode(vulnText)
	if err != nil {
		return wrapper
		fmt.Println("Error decoding hex input.")
	}
	plainVulns, err := decryptString(sarpConfig.Password, vulnText)
	if err != nil {
		fmt.Println(err)
		return wrapper
	}
	splitVulns := strings.Split(plainVulns, "|-SP-|")
	scored, err := strconv.Atoi(splitVulns[0])
	wrapper.VulnsScored = scored
	if err != nil {
		panic(err)
	}
 	total, err := strconv.Atoi(splitVulns[1])
	wrapper.VulnsTotal = total
	if err != nil {
		panic(err)
	}
	splitVulns = splitVulns[2:len(splitVulns)-2]
	fmt.Println("splitvulns", splitVulns)
	for _, vuln := range splitVulns {
		splitVuln := strings.Split(vuln, "-")
		fmt.Println("splitVuln", splitVuln)
		fmt.Println("len", len(splitVuln))
		fmt.Println("some reasson, keep going...")
		if len(splitVuln) != 2 {
			panic(errors.New("Invalid vuln input"))
		}
		vulnText := strings.TrimSpace(splitVuln[0])
		splitVuln = strings.Split(strings.TrimSpace(splitVuln[1]), " ")
		if len(splitVuln) != 2 {
			panic(errors.New("Invalid vuln input"))
		}
		vulnPoints, err := strconv.Atoi(splitVuln[0])
		if err != nil {
			panic(errors.New("Invalid vuln input"))
		}
		fmt.Println("appending", vulnText, vulnPoints)
		wrapper.VulnItems = append(wrapper.VulnItems, vulnItem{VulnText: vulnText, VulnPoints: vulnPoints})
	}
	fmt.Println("wrapper", wrapper)
	return wrapper
}

// encryptString takes a password and a plaintext and returns an encrypted byte
// sequence (as a string). It uses AES-GCM with a 12-byte IV (as is
// recommended). The IV is prefixed to the string.
func encryptString(password, plaintext string) string {

	// Create a sha256sum hash of the password provided.
	hasher := sha256.New()
	hasher.Write([]byte(password))
	key := hasher.Sum(nil)

	// Pad plaintext to be a 16-byte block.
	paddingArray := make([]byte, (aes.BlockSize - len(plaintext)%aes.BlockSize))
	for char := range paddingArray {
		paddingArray[char] = 0x20 // Padding with space character.
	}
	plaintext = plaintext + string(paddingArray)
	if len(plaintext)%aes.BlockSize != 0 {
		panic("Plaintext is not a multiple of block size!")
	}

	// Create cipher block with key.
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// Generate nonce.
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}

	// Create NewGCM cipher.
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	// Encrypt and seal plaintext.
	ciphertext := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)
	ciphertext = []byte(fmt.Sprintf("%s%s", nonce, ciphertext))

	return string(ciphertext)
}

// decryptString takes a password and a ciphertext and returns a decrypted
// byte sequence (as a string). The function uses typical AES-GCM.
func decryptString(password, ciphertext string) (string, error) {

	hasher := sha256.New()
	hasher.Write([]byte(password))
	key := hasher.Sum(nil)

	iv := []byte(ciphertext[:12])
	ciphertext = ciphertext[12:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := aesgcm.Open(nil, iv, []byte(ciphertext), nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func hexEncode(inputString string) string {
	return hex.EncodeToString([]byte(inputString))
}

func hexDecode(inputString string) (string, error) {
	result, err := hex.DecodeString(inputString)
	if err != nil {
		return "", err
	}
	return string(result), nil
}
