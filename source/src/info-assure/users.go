package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

//
func routeUsersGet(c *gin.Context) {

	accountType := getAccountType(c)
	if accountType != ADMIN {
		c.Redirect(http.StatusTemporaryRedirect, "/logout")
		return
	}

	data, err := getUsers()
	if err != nil {
		log.Printf("Error loading users: %v\n", err)
		goToErrorPage(c, "Unable to load users")
		return
	}

	c.HTML(http.StatusOK, "users", gin.H{"users": data})
}
