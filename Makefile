all:
	go install github.com/YaoZengzeng/yustack/sample/tun_icmp_echo
	go install github.com/YaoZengzeng/yustack/sample/tun_udp_echo

gofmt:
	@./hack/verify-gofmt.sh
