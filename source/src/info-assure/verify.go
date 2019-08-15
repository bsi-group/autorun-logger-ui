package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/dgryski/dgoogauth"
	"github.com/gin-gonic/gin"
)

//
func routeVerifyGet(c *gin.Context) {

	c.HTML(http.StatusOK, "verify", gin.H{"code": ""})
}

//
func routeVerifyPost(c *gin.Context) {

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
	log.Println(token)
	log.Println(ok)
	log.Println(err)

	// if the token is invalid or expired
	if ok == false {
		c.HTML(http.StatusOK, "verify", gin.H{"code": ""})
	} else {
		c.Redirect(http.StatusFound, "/alerts")
	}
}
