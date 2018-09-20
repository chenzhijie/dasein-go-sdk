package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/ontio/ontology-crypto/keypair"
	"golang.org/x/crypto/scrypt"
)

type SymmetricScheme byte

const (
	AES SymmetricScheme = iota
)

var names []string = []string{
	"AES",
}

func GetScheme(name string) (SymmetricScheme, error) {
	for i, v := range names {
		if strings.ToUpper(v) == strings.ToUpper(name) {
			return SymmetricScheme(i), nil
		}
	}
	return 0, errors.New("unknown symmetric scheme " + name)
}
func kdf(pwd []byte, salt []byte) (dKey []byte, err error) {
	param := keypair.GetScryptParameters()
	if param.DKLen < 32 {
		err = errors.New("derived key length too short")
		return nil, err
	}
	// Derive the encryption key
	dKey, err = scrypt.Key([]byte(pwd), salt, param.N, param.R, param.P, param.DKLen)
	return dKey, err
}
func randomBytes(length int) ([]byte, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// AESEncryptFile encrypt file and save file locally
func AESEncryptFile(file string, password string, out string) error {
	salt, err := randomBytes(16)
	if err != nil {
		return errors.New("salt generate error")
	}
	dKey, err := kdf([]byte(password), salt)
	if err != nil {
		return err
	}
	nonce := dKey[:16]
	eKey := dKey[len(dKey)-16:]

	inFile, err := os.Open(file)
	if err != nil {
		return err
	}
	defer inFile.Close()

	block, err := aes.NewCipher(eKey)
	if err != nil {
		return err
	}
	stream := cipher.NewOFB(block, nonce)
	outFile, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer outFile.Close()

	writer := &cipher.StreamWriter{S: stream, W: outFile}
	if _, err := io.Copy(writer, inFile); err != nil {
		return err
	}
	tag := []byte{}
	for _, v := range salt {
		tag = append(tag, v)
	}
	encAlg := make([]byte, 16)
	encAlg[15] = byte(AES)
	for _, v := range encAlg {
		tag = append(tag, v)
	}
	outFile.Write(tag)
	return nil
}

func AESEncryptFileReader(inFile *os.File, password string) (io.Reader, error) {
	salt, err := randomBytes(16)
	if err != nil {
		return nil, errors.New("salt generate error")
	}
	dKey, err := kdf([]byte(password), salt)
	if err != nil {
		return nil, err
	}
	nonce := dKey[:16]
	eKey := dKey[len(dKey)-16:]

	block, err := aes.NewCipher(eKey)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewOFB(block, nonce)
	reader := &cipher.StreamReader{S: stream, R: inFile}
	tag := []byte{}
	for _, v := range salt {
		tag = append(tag, v)
	}
	encAlg := make([]byte, 16)
	encAlg[15] = byte(AES)
	for _, v := range encAlg {
		tag = append(tag, v)
	}
	r := io.MultiReader(reader, bytes.NewReader(tag))
	return r, nil
}

func AESDecryptFile(file string, password string, out string) error {
	inFile, err := os.Open(file)
	if err != nil {
		return err
	}
	defer inFile.Close()
	extData := make([]byte, 32)
	fileSize, _ := inFile.Stat()
	inFile.ReadAt(extData, fileSize.Size()-32)
	err = os.Truncate(file, fileSize.Size()-32)
	if err != nil {
		return err
	}
	salt := extData[:16]
	dKey, err := kdf([]byte(password), salt)
	if err != nil {
		return err
	}
	nonce := dKey[:16]
	eKey := dKey[len(dKey)-16:]
	encAlg := extData[31]
	if encAlg != byte(AES) {
		return errors.New("unknown algorithm")
	}
	block, err := aes.NewCipher(eKey)
	if err != nil {
		return err
	}

	stream := cipher.NewOFB(block, nonce)
	outFile, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer outFile.Close()
	reader := &cipher.StreamReader{S: stream, R: inFile}
	// Copy the input file to the output file, decrypting as we go.
	if _, err := io.Copy(outFile, reader); err != nil {
		return err
	}
	return nil
}

func AESDecrptyFileWriter(inFile *os.File, password string) (io.Writer, error) {
	extData := make([]byte, 32)
	fileSize, _ := inFile.Stat()
	inFile.ReadAt(extData, fileSize.Size()-32)
	err := os.Truncate(inFile.Name(), fileSize.Size()-32)
	if err != nil {
		return nil, err
	}
	salt := extData[:16]
	dKey, err := kdf([]byte(password), salt)
	if err != nil {
		return nil, err
	}
	nonce := dKey[:16]
	eKey := dKey[len(dKey)-16:]
	encAlg := extData[31]
	if encAlg != byte(AES) {
		return nil, errors.New("unkown algorithm")
	}
	block, err := aes.NewCipher(eKey)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewOFB(block, nonce)
	writer := &cipher.StreamWriter{S: stream}
	return writer, nil
}
