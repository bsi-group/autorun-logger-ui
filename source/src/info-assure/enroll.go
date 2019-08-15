package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/dgryski/dgoogauth"
	"github.com/gin-gonic/gin"
)

//
func routeEnrollGet(c *gin.Context) {

	userID := getCookieInt64Value(c, "user_id")
	if userID == -1 {
		log.Println("Error checking session: invalid user ID")
		goToErrorPage(c, "Unable perform enroll")
		return
	}

	u, err := NewUserByID(userID)
	if err != nil {
		log.Printf("Error loading user details: %v\n", err)
		goToErrorPage(c, "Unable perform enroll")
		return
	}

	qr, err := generateQr(u.MfaSecret)
	if err != nil {
		goToErrorPage(c, "Unable to perform login")
		return
	}

	c.HTML(http.StatusOK, "enroll", gin.H{"qr": qr, "code": ""})
}

//
func routeEnrollPost(c *gin.Context) {

	userID := getCookieInt64Value(c, "user_id")
	if userID == -1 {
		log.Println("Error checking session: invalid user ID")
		goToErrorPage(c, "Unable perform enroll verify")
		return
	}

	u, err := NewUserByID(userID)
	if err != nil {
		log.Printf("Error loading user details for enroll verification: %v\n", err)
		goToErrorPage(c, "Unable perform enroll")
		return
	}

	token := c.Request.FormValue("code")

	// setup the one-time-password configuration.
	otpConfig := &dgoogauth.OTPConfig{
		Secret:      strings.TrimSpace(u.MfaSecret),
		WindowSize:  3,
		HotpCounter: 0,
	}

	// Validate token
	ok, err := otpConfig.Authenticate(strings.TrimSpace(token))

	// if the token is invalid or expired
	if ok == false {

		qr, err := generateQr(u.MfaSecret)
		if err != nil {
			goToErrorPage(c, "Unable to perform login")
			return
		}

		c.HTML(http.StatusOK, "enroll", gin.H{"qr": qr, "code": ""})
	} else {
		u.MfaSet = true
		err = u.Update()
		if err != nil {
			log.Printf("Error updating user for enroll verification: %v\n", err)
			goToErrorPage(c, "Unable perform enroll")
			return
		}

		session, _ := sessionStore.Get(c.Request, APP_NAME)
		session.Values["mfa_set"] = u.MfaSet
		err = session.Save(c.Request, c.Writer)
		if err != nil {
			log.Printf("Error saving user session (enroll): %v\n", err)
			goToErrorPage(c, "Unable perform enroll")
			return
		}

		c.Redirect(http.StatusFound, "/alerts")
	}
}
