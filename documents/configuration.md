# Configuration

There are various configuration options for the server application. This document details the configuration options and the required values:

- database_server: The host name or IP address of the PostgreSQL database server
- database_name: Name of the database (arl)
- database_user:  Database server user name (postgres)
- database_password: Password for the database user
- http_ip: IP address of the interface for the HTTPS API. Use 0.0.0.0 to access on all interfaces
- http_port: Port for the HTTPS server. Use the **arl-setbind.sh** file to allow lower port access such as port 80
- debug: Show each HTTPS request in the logs (true/false)
- static_dir: Directory used to store the HTML static resources
- template_dir: Directory used to store the HTML templates
- summary_dir: Directory used to store the automatically generated summary files
