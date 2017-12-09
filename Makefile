all:
	go install github.com/YaoZengzeng/yustack/sample/tun_icmp_echo
	go install github.com/YaoZengzeng/yustack/sample/tun_udp_echo
	go install github.com/YaoZengzeng/yustack/sample/tun_tcp_echo
	go install github.com/YaoZengzeng/yustack/sample/tun_tcp_connect

gofmt:
	@./hack/verify-gofmt.sh
