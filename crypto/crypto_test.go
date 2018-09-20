package crypto

import (
	"crypto/cipher"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

var (
	testFile      = "test.txt"
	encryptedFile = "encrypted-file.txt"
	decryptedFile = "decrypted-file.txt"
	password      = "123456"
)

func TestAESEncryptAndSaveFile(t *testing.T) {
	err := AESEncryptFile(testFile, password, encryptedFile)
	if err != nil {
		t.Errorf("err:%s", err)
	}
	fmt.Println("encrypt file success")
}

func TestAESDecryptAndSaveFile(t *testing.T) {
	err := AESDecryptFile(encryptedFile, password, decryptedFile)
	if err != nil {
		t.Errorf("err:%s", err)
	}
	fmt.Println("decrypt file success")
}

func TestAESEncryptFileReader(t *testing.T) {
	inFile, err := os.Open(testFile)
	if err != nil {
		t.Errorf("err:%s", err)
		return
	}
	defer inFile.Close()
	r, err := AESEncryptFileReader(inFile, password)
	if r == nil {
		t.Errorf("err:%s", err)
		return
	}
	if err != nil {
		t.Errorf("err:%s", err)
		return
	}
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		t.Errorf("err:%s", err)
		return
	}
	ioutil.WriteFile(encryptedFile, buf, 0666)
	fmt.Printf("finish write encrypted file from reader\n")
}

func TestAESDecryptFileWriter(t *testing.T) {
	inFile, err := os.Open(encryptedFile)
	if err != nil {
		t.Errorf("err:%s", err)
		return
	}
	defer inFile.Close()
	w, err := AESDecrptyFileWriter(inFile, password)
	outFile, err := os.OpenFile(decryptedFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		t.Errorf("err:%s", err)
		return
	}
	defer outFile.Close()
	streamWriter := w.(*cipher.StreamWriter)
	streamWriter.W = outFile
	if _, err := io.Copy(w, inFile); err != nil {
		t.Errorf("err:%s", err)
		return
	}
	fmt.Println("finish write decrypted file from writer")
}
