## Go IP_FREEBIND HTTP Proxy

A Go implementation of an HTTP Proxy that leverages the `IP_FREEBIND` socket option to randomize the source IP address for outgoing HTTP requests.
This proxy is compatible with both IPv6 and IPv4 subnets.

### Features
* **Randomized Source IPs**: Makes HTTP requests with randomized IPs from a specified subnet
* **IPv6 & IPv4 Support**: Works seamlessly across IPv6 and IPv4 environments
* **Flexible Usage**: Can be used as either a standalone binary or embedded as a library

### Requirements
* Linux (due to the reliance on `IP_FREEBIND`)
* Correctly assigned and configured network subnet on the machine

### Setup

Assume your ISP has assigned the subnet `2a00:1450:4001:81b::/64` to your server.

In order to make use of freebinding, you first need to configure the
[Linux AnyIP kernel feature](https://git.kernel.org/cgit/linux/kernel/git/torvalds/linux.git/commit/?id=ab79ad14a2d51e95f0ac3cef7cd116a57089ba82) 
in order to be able to bind a socket to an arbitrary IP address from this subnet as follows:

```shell
ip -6 route add local 2a00:1450:4001:81b::/64 dev lo
```

### Installation

* As a Standalone Binary:
    ```shell
    go install github.com/codercms/freebind-proxy/cmd/freebind-proxy@latest
    ```

* As an Embeddable Library:
    ```shell
    go get github.com/codercms/freebind-proxy
    ```

### Usage

* **Run as a Binary**: After installation, execute the binary from your $GOPATH/bin:
    ```shell
    freebind-proxy -net 2a00:1450:4001:81b::/64
    ```
    See `-help` for more options


* **Embed as a Library**: Import and use in your Go project:
    ```go
    import "github.com/codercms/freebind-proxy/proxy"
    ```

### Refs
* https://github.com/blechschmidt/freebind
* https://oswalt.dev/2022/02/non-local-address-binds-in-linux/
* https://blog.widodh.nl/2016/04/anyip-bind-a-whole-subnet-to-your-linux-machine/
