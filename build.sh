rm ./bin/*
# # Per Windows
# GOOS=windows GOARCH=amd64 go build -o "./bin/perfect-menu_print_orders-amd64.exe" "./cmd/"
# shasum -a 256 "./bin/perfect-menu_print_orders-amd64.exe" > "./bin/perfect-menu_print_orders-amd64.exe.sha256"
# gzip -k -f "./bin/perfect-menu_print_orders-amd64.exe"

# Per Linux
GOOS=linux GOARCH=amd64 go build -o "./bin/perfect-menu_print_orders-amd64-linux" "./cmd/"
tar -czf "./bin/perfect-menu_print_orders-amd64-linux.tar.gz" -C ./bin perfect-menu_print_orders-amd64-linux -C .. templates/
shasum -a 256 "./bin/perfect-menu_print_orders-amd64-linux.tar.gz" > "./bin/perfect-menu_print_orders-amd64-linux.tar.gz.sha256"

# For Raspberry Pi (ARM)
GOOS=linux GOARCH=arm64 go build -o "./bin/perfect-menu_print_orders-arm64-linux" "./cmd/"
tar -czf "./bin/perfect-menu_print_orders-arm64-linux.tar.gz" -C ./bin perfect-menu_print_orders-arm64-linux -C .. templates/
shasum -a 256 "./bin/perfect-menu_print_orders-arm64-linux.tar.gz" > "./bin/perfect-menu_print_orders-arm64-linux.tar.gz.sha256" 

# Per macOS
GOOS=darwin GOARCH=amd64 go build -o "./bin/perfect-menu_print_orders-amd64-macos" "./cmd/"
tar -czf "./bin/perfect-menu_print_orders-amd64-macos.tar.gz" -C ./bin perfect-menu_print_orders-amd64-macos -C .. templates/
shasum -a 256 "./bin/perfect-menu_print_orders-amd64-macos.tar.gz" > "./bin/perfect-menu_print_orders-amd64-macos.tar.gz.sha256"
