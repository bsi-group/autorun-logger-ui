package main

// Stores the YAML config file data
type Config struct {
	HttpIp				string	`yaml:"http_ip"`
	HttpPort			int16	`yaml:"http_port"`
	DatabaseServer		string 	`yaml:"database_server"`
	DatabaseName		string 	`yaml:"database_name"`
	DatabaseUser		string 	`yaml:"database_user"`
	DatabasePassword    string 	`yaml:"database_password"`
	Debug				bool	`yaml:"debug"`
	StaticDir 			string	`yaml:"static_dir"`
	TemplateDir 		string	`yaml:"template_dir"`
	ExportDir			string	`yaml:"export_dir"`
}