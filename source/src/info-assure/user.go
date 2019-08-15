package main

import (
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

//
type User struct {
	ID                     int64  `db:"id"`
	Username               string `db:"username"`
	Name                   string `db:"name"`
	Password               string
	PasswordVerify         string
	PasswordHash           string    `db:"password_hash"`
	AccountType            int16     `db:"account_type"`
	AccountTypeString      string    `db:"-"`
	TimestampCreated       time.Time `db:"timestamp_created"`
	TimestampCreatedString string    `db:"-"`
	LoginAttempts          int16     `db:"login_attempts"`
	Locked                 bool      `db:"is_locked"`
	MfaSecret              string    `db:"mfa_secret"`
	MfaSet                 bool      `db:"is_mfa_set"`
}

//
func NewUserByUsername(username string) (*User, error) {

	u := new(User)
	err := db.
		Select("*").
		From("users").
		Where("username = $1", username).
		QueryStruct(u)

	if err == sql.ErrNoRows {
		return u, errors.New("User does not exist")
	}
	if err != nil {
		return u, err
	}

	return u, nil
}

//
func NewUserByID(id int64) (*User, error) {

	u := new(User)
	err := db.
		Select("*").
		From("users").
		Where("id = $1", id).
		QueryStruct(u)

	if err == sql.ErrNoRows {
		return u, errors.New("User does not exist")
	}
	if err != nil {
		return u, err
	}

	return u, nil
}

//
func (u *User) Add() error {

	hash, err := getPasswordHash(u.Password)
	if err != nil {
		log.Println(err)
		return err
	}

	// Generate random string for Google 2FA
	random := generateRandomString(16)
	// For Google Authenticator purpose: https://github.com/google/google-authenticator/wiki/Key-Uri-Format
	secret := base32.StdEncoding.EncodeToString([]byte(random))

	u.PasswordHash = hash

	err = db.
		InsertInto("users").
		Columns("username", "name", "password_hash", "account_type", "timestamp_created", "login_attempts", "is_locked", "mfa_secret", "is_mfa_set").
		Values(u.Username, u.Name, u.PasswordHash, u.AccountType, time.Now(), 0, false, secret, false).
		Returning("*").
		QueryStruct(u)

	fmt.Printf("%v", u)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}

		return err
	}

	return nil
}

//
func (u *User) Update() error {

	_, err := db.
		Update("users").
		Set("username", u.Username).
		Set("account_type", u.AccountType).
		Set("name", u.Name).
		Set("mfa_secret", u.MfaSecret).
		Set("is_mfa_set", u.MfaSet).
		Where("id = $1", u.ID).
		Exec()

	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return nil
		}

		return err
	}

	return nil
}

//
func (u *User) Delete() error {

	_, err := db.
		DeleteFrom("users").
		Where("id = $1", u.ID).
		Exec()

	if err == sql.ErrNoRows {
		return errors.New("User does not exist")
	}
	if err != nil {
		return err
	}

	return nil
}

//
func (u *User) Beautify() {

	u.AccountTypeString = AccountType(u.AccountType).String()
}

//
func (u *User) SetPassword(password string) bool {

	hash, err := getPasswordHash(password)
	log.Println(password)
	log.Println(hash)
	if err != nil {
		log.Printf("Error generating password hash: %v", err)
		return false
	}

	log.Println(u.ID)

	_, err = db.
		Update("users").
		Set("password_hash", hash).
		Where("id = $1", u.ID).
		Exec()

	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return true
		}

		log.Printf("Error reseting user password: %v\n", err)
		return false
	}

	return true
}

//
func (u *User) Exists() (bool, error) {

	row := db.DB.QueryRow("SELECT count(1) from users where username = $1", u.Username)
	var count int
	err := row.Scan(&count)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if count > 0 {
		return true, nil
	}

	return false, nil
}

//
func (u *User) Validate(ignoreCreds bool) error {

	if len(u.Username) < 3 {
		return errors.New("Username too short (Minimum 3)")
	}

	if len(u.Username) > 25 {
		return errors.New("Username too long (Maximum 25)")
	}

	if ignoreCreds == false {
		// Ensure we have matching passwords
		if u.Password != u.PasswordVerify {
			return errors.New("New password does not match verify password")
		}

		if len(u.Password) > 50 {
			return errors.New("Password too long (Maximum 50)")
		}

		// found, err := checkCommonPasswords(u.Password)
		// if err != nil {
		// 	return errors.New("Unable to check password")
		// }
		// if found == true {
		// 	return errors.New("Password too common")
		// }
	}

	return nil
}

//
func (u *User) ValidatePassword() error {

	// Ensure we have matching passwords
	if u.Password != u.PasswordVerify {
		return errors.New("New password does not match verify password")
	}

	if len(u.Password) > 50 {
		return errors.New("Password too long (Maximum 50)")
	}

	found, err := checkCommonPasswords(u.Password)
	if err != nil {
		return errors.New("Unable to check password")
	}
	if found == true {
		return errors.New("Password too common")
	}

	return nil
}

//
func checkCommonPasswords(password string) (bool, error) {

	row := db.DB.QueryRow("SELECT count(1) from common_passwords where password = $1", password)
	var count int
	err := row.Scan(&count)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if count > 0 {
		return true, nil
	}

	return false, nil
}

//
func (u User) CheckPassword() error {

	var t User

	err := db.
		Select("id", "password_hash").
		From("users").
		Where("username = $1", u.Username).
		QueryStruct(&t)

	if err == sql.ErrNoRows {
		return errors.New("User does not exist")
	}
	if err != nil {
		log.Printf("Error loading user: %v\n", err)
		return errors.New("Error loading user")
	}
	if checkPasswordHash(u.Password, t.PasswordHash) != nil {
		return errors.New("Password is incorrect")
	}

	return nil
}

//
func (u *User) IncrementLoginAttempts() {

	u.LoginAttempts = u.LoginAttempts + 1

	_, err := db.
		Update("users").
		Set("login_attempts", u.LoginAttempts).
		Where("id = $1", u.ID).
		Exec()

	if u.LoginAttempts >= config.MaxFailedLogins {
		u.Lock()
		u.ResetLoginAttempts()
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return
		}

		log.Printf("Error incrementing user logins: %v\n", err)
	}
}

//
func (u *User) ResetLoginAttempts() {

	_, err := db.
		Update("users").
		Set("login_attempts", 0).
		Where("id = $1", u.ID).
		Exec()

	if err != nil {
		if err == sql.ErrNoRows {
			return
		}

		log.Printf("Error resetting user login attempts: %v\n", err)
	}
}

//
func (u *User) Lock() bool {

	_, err := db.
		Update("users").
		Set("is_locked", true).
		Where("id = $1", u.ID).
		Exec()

	if err != nil {
		log.Printf("Error locking user: %v\n", err)
		return false
	}

	u.Locked = true
	return true
}

//
func (u *User) Unlock() bool {

	_, err := db.
		Update("users").
		Set("is_locked", false).
		Set("login_attempts", 0).
		Where("id = $1", u.ID).
		Exec()

	if err != nil {
		log.Printf("Error unlocking user: %v\n", err)
		return false
	}

	u.Locked = false
	return true
}

// ***** Routing Methods ******************************************************

//
func routeUserNewGet(c *gin.Context) {

	accountType := getAccountType(c)
	if accountType != ADMIN {
		c.Redirect(http.StatusTemporaryRedirect, "/logout")
		return
	}

	c.HTML(http.StatusOK, "user", gin.H{"endpoint": "new", "title": "New User", "u": new(User)})
}

//
func routeUserNewPost(c *gin.Context) {

	accountType := getAccountType(c)
	if accountType != ADMIN {
		c.Redirect(http.StatusTemporaryRedirect, "/logout")
		return
	}

	u := new(User)
	u.Username = strings.TrimSpace(c.PostForm("username"))
	u.Name = strings.TrimSpace(c.PostForm("name"))
	u.AccountType = convertStringToInt16(strings.TrimSpace(c.PostForm("account_type")))

	// Ensure that the user does not already exist
	exists, err := u.Exists()
	if err != nil {
		log.Printf("Error checking new user existance: %v\n", err)
		goToErrorPage(c, "Unable to add user")
		return
	}

	if exists == true {
		c.HTML(http.StatusOK, "user", gin.H{"endpoint": "new", "u": u,
			"message": template.HTML(fmt.Sprintf(ALERT_YELLOW, "User already exists"))})
		return
	}

	err = u.Validate(true)
	if err != nil {
		c.HTML(http.StatusOK, "user", gin.H{"endpoint": "new", "u": u,
			"message": template.HTML(fmt.Sprintf(ALERT_YELLOW, err.Error()))})
		return
	}

	err = u.Add()
	if err != nil {
		log.Printf("Error adding user: %v\n", err)
		goToErrorPage(c, "Unable to add user")
		return
	}

	password := generateRandomString(8)

	if u.SetPassword(password) == false {
		log.Printf("Error setting new user password: %v\n", err)
		goToErrorPage(c, "Unable to set user password")
		return
	}

	u = new(User)
	c.HTML(http.StatusOK, "user", gin.H{"endpoint": "new", "title": "New User", "u": u,
		"message": template.HTML(fmt.Sprintf(ALERT_GREEN, "User added (Password: "+password+")"))})
}
