package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/sessions"
)

// AuthorizeMiddleware is used to authorize a request for a certain end-point group.
func AuthorizeMiddleware() gin.HandlerFunc {

	return func(c *gin.Context) {

		session, err := sessionStore.Get(c.Request, APP_NAME)
		if err != nil {
			c.Abort()
			c.Redirect(http.StatusFound, "/")
			return
		}

		if session.Values["authed"] == nil {
			c.Abort()
			c.Redirect(http.StatusFound, "/")
			return
		}

		ret := session.Values["authed"].(bool)
		if ret == false {
			c.Abort()
			c.Redirect(http.StatusFound, "/")
			return
		}

		userID := getCookieInt64Value(c, "user_id")
		if userID == -1 {
			c.Abort()
			c.Redirect(http.StatusFound, "/")
			return
		}

		if c.Request.URL.Path != "/enroll" {
			ret = session.Values["mfa_set"].(bool)
			if ret == false {
				c.Abort()
				c.Redirect(http.StatusFound, "/enroll")
				return
			}
		}

		accountType := getAccountType(c)
		c.Set("account_type", AccountType(accountType).String())
		c.Next()
	}
}

//
func routeLogonGet(c *gin.Context) {

	c.HTML(http.StatusOK, "logon", gin.H{})
}

//
func routeLogonPost(c *gin.Context) {

	u := new(User)
	u.Username = c.PostForm("username")
	exists, err := u.Exists()
	if err != nil {
		log.Printf("Error checking user existance: %v\n", err)
		goToErrorPage(c, "Unable to perform login")
		return
	}

	if exists == false {
		log.Printf("Error user does not exist: %v\n", u.Username)
		c.HTML(http.StatusOK, "logon", gin.H{"message": template.HTML(fmt.Sprintf(ALERT_YELLOW, "User does not exist"))})
		return
	}

	u, err = NewUserByUsername(u.Username)
	if err != nil {
		log.Printf("Error loading user details: %v\n", err)
		goToErrorPage(c, "Unable to perform login")
		return
	}

	if u.Locked == true {
		log.Printf("Error user locked: %v\n", u.Username)
		c.HTML(http.StatusOK, "logon", gin.H{"message": template.HTML(fmt.Sprintf(ALERT_YELLOW, "Account locked"))})
		return
	}

	u.Password = c.PostForm("password")

	err = u.CheckPassword()
	if err != nil {
		log.Printf("Error checking password: %v\n", err)

		if strings.Contains(err.Error(), "Password is incorrect") {
			u.IncrementLoginAttempts()
		}

		//c.HTML(http.StatusOK, "password", getTemplateData(c, ROUTE_LOGON,
		c.HTML(http.StatusOK, "logon", gin.H{"message": template.HTML(fmt.Sprintf(ALERT_YELLOW, err))})
		return
	}

	u.ResetLoginAttempts()

	session, _ := sessionStore.Get(c.Request, APP_NAME)
	session.Options = &gorilla.Options{MaxAge: config.InactiveSessionTimeoutSeconds}
	session.Values["authed"] = true
	session.Values["user_id"] = u.ID
	session.Values["username"] = u.Username
	session.Values["account_type"] = u.AccountType
	session.Values["mfa_set"] = u.MfaSet
	err = session.Save(c.Request, c.Writer)
	if err != nil {
		log.Printf("Error saving user session (logon): %v\n", err)
		goToErrorPage(c, "Unable to perform login")
		return
	}

	if u.MfaSet == false {
		c.Redirect(http.StatusFound, "/enroll")
	} else {
		c.Redirect(http.StatusFound, "/verify")
	}
}

//
func routeLogout(c *gin.Context) {

	session, err := sessionStore.Get(c.Request, APP_NAME)
	if err == nil {
		session.Options = &gorilla.Options{MaxAge: -1}
		session.Values["authed"] = false
		err = session.Save(c.Request, c.Writer)
		if err != nil {
			log.Printf("Error logging out session: %v\n", err)
		}
	}

	c.Redirect(http.StatusFound, "/")
}
