GOOS=darwin GOARCH=arm64 go build -o javan-codecov-m1 . 
GOOS=windows GOARCH=amd64 go build -o javan-codecov-win.exe .
GOOS=linux GOARCH=amd64 go build -o javan-codecov .