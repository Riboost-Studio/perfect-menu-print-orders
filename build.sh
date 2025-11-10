rm ./bin/*
# # Per Windows
# GOOS=windows GOARCH=amd64 go build -o "./bin/perfect-menu_print_orders-amd64.exe" "./src/"
# shasum -a 256 "./bin/perfect-menu_print_orders-amd64.exe" > "./bin/perfect-menu_print_orders-amd64.exe.sha256"
# gzip -k -f "./bin/perfect-menu_print_orders-amd64.exe"

# Per Linux
GOOS=linux GOARCH=amd64 go build -o "./bin/perfect-menu_print_orders-amd64-linux" "./src/"
shasum -a 256 "./bin/perfect-menu_print_orders-amd64-linux" > "./bin/perfect-menu_print_orders-amd64-linux.sha256"
gzip -k -f "./bin/perfect-menu_print_orders-amd64-linux"

# For Raspberry Pi (ARM)
GOOS=linux GOARCH=arm64 go build -o "./bin/perfect-menu_print_orders-arm64-linux" "./src/"
shasum -a 256 "./bin/perfect-menu_print_orders-arm64-linux" > "./bin/perfect-menu_print_orders-arm64-linux.sha256"
gzip -k -f "./bin/perfect-menu_print_orders-arm64-linux" 

# Per macOS
GOOS=darwin GOARCH=amd64 go build -o "./bin/perfect-menu_print_orders-amd64-macos" "./src/"
shasum -a 256 "./bin/perfect-menu_print_orders-amd64-macos" > "./bin/perfect-menu_print_orders-amd64-macos.sha256"
gzip -k -f "./bin/perfect-menu_print_orders-amd64-macos"