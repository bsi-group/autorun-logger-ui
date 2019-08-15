package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/png"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	util "github.com/woanware/goutil"
	"golang.org/x/crypto/bcrypt"
	"rsc.io/qr"
)

//
func generateRandomString(length int) string {

	dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	var bytes = make([]byte, length)
	rand.Read(bytes)
	for k, v := range bytes {
		bytes[k] = dictionary[v%byte(len(dictionary))]
	}
	return string(bytes)
}

//
func getPasswordHash(password string) (string, error) {

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

//
func checkPasswordHash(password string, hash string) error {

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func splitRegKey(data string) string {

	parts := strings.Split(data, "\\")
	if len(parts) == 1 {
		return data
	}

	return parts[0] + "..." + parts[len(parts)-1]
}

//
func processIntParameter(data string) (int, bool) {

	if len(data) > 0 {
		if util.IsNumber(data) == true {
			return util.ConvertStringToInt(data), true
		}
	}

	return -1, false
}

//
func processInt64Parameter(data string) (int64, bool) {

	if len(data) > 0 {
		if util.IsNumber(data) == true {
			return util.ConvertStringToInt64(data), true
		}
	}

	return -1, false
}

//
func getCookieInt64Value(c *gin.Context, cookieKey string) int64 {

	session, err := sessionStore.Get(c.Request, APP_NAME)
	if err != nil {
		return -1
	}

	if session.Values[cookieKey] == nil {
		return -1
	}

	return session.Values[cookieKey].(int64)
}

//
func getCookieInt16Value(c *gin.Context, cookieKey string) int16 {

	session, err := sessionStore.Get(c.Request, APP_NAME)
	if err != nil {
		return -1
	}

	if session.Values[cookieKey] == nil {
		return -1
	}

	return session.Values[cookieKey].(int16)
}

// //
// func getCookieStringValue(c *gin.Context, cookieKey string) string {

// 	session, err := sessionStore.Get(c.Request, APP_NAME)
// 	if err != nil {
// 		return -1
// 	}

// 	if session.Values[cookieKey] == nil {
// 		return -1
// 	}

// 	return session.Values[cookieKey].(string)
// }

//
func getAccountType(c *gin.Context) AccountType {

	return AccountType(getCookieInt16Value(c, "account_type"))
}

//
func processCurrentPageNumber(data string, mode string) int {

	if len(data) == 0 {
		return 0
	}

	if util.IsNumber(data) == false {
		return 0
	}

	currentPageNumber := util.ConvertStringToInt(data)

	if mode == "first" {
		return 0
	}

	if mode == "next" {
		currentPageNumber++
		return currentPageNumber
	}

	if mode == "previous" {
		currentPageNumber--
		return currentPageNumber
	}

	if currentPageNumber < 0 {
		return 0
	}

	return 0
}

//
func goToErrorPage(c *gin.Context, message string) {

	//c.HTML(http.StatusInternalServerError, "error", getTemplateData(c, ROUTE_ERROR, gin.H{"message": message}))
	//c.HTML(http.StatusInternalServerError, "error", gin.H{})

	c.Redirect(http.StatusFound, "/")
}

func convertStringToInt16(v string) int16 {

	ret, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return int16(ret)
}

//
func convertInt64ToString(data int64) string {

	return strconv.FormatInt(data, 10)
}

//
func generateQr(secret string) (string, error) {

	// Authentication link: https://github.com/google/google-authenticator/wiki/Key-Uri-Format
	authLink := "otpauth://totp/AutoRunLogger?secret=" + secret + "&issuer=AutoRunLogger"
	// Encode authLink to QR codes: https://godoc.org/code.google.com/p/rsc/qr#Level
	// qr.H = 65% redundant level
	code, err := qr.Encode(authLink, qr.H)
	if err != nil {
		log.Printf("Error generating enroll QR: %v\n", err)
		return "", err
	}

	// convert byte to image for saving to file
	img, _, err := image.Decode(bytes.NewReader(code.PNG()))
	if err != nil {
		log.Printf("Error decoding QR png: %v\n", err)
		return "", err
	}

	// Resize the image e.g. smaller, as it was masssssiiiivvveee
	img = imaging.Resize(img, 234, 234, imaging.Lanczos)
	buf := new(bytes.Buffer)
	err = png.Encode(buf, img)
	if err != nil {
		log.Printf("Error encoding resized QR png: %v\n", err)
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
