-- Canary-Go server configuration (executed as Lua, like the C++ server).

serverName = "Canary-Go"
ip = "127.0.0.1"

loginProtocolPort = 7171
gameProtocolPort = 7172
statusProtocolPort = 7171

dataPackDirectory = "../data-otservbr-global"
coreDirectory = "../data"

-- MariaDB connection (shared with MyAAC and the login-server).
mysqlHost = "0.0.0.0"
mysqlPort = 3306
mysqlUser = "canary"
mysqlPass = "canary"
mysqlDatabase = "canary"

motd = "Welcome to Canary-Go! A Go migration of the Canary server."
allowOldProtocol = true

rsaKeyFile = "key.pem"

-- Spawn field (synthetic map) generated around the town temple until OTBM
-- loading is enabled.
worldFile = ""
