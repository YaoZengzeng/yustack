all:
	go install github.com/YaoZengzeng/yustack/sample

gofmt:
	@./hack/verify-gofmt.sh
