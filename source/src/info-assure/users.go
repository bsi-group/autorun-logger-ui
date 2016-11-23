package main

type Users struct {
	Data []User	`yaml:"users"`
}

type User struct {
	UserName	string	`yaml:"user_name"`
	FullName	string	`yaml:"full_name"`
	Password	string	`yaml:"password"`
}
