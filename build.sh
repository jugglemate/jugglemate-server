CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./jchat-ai main.go

mv jchat-ai build/