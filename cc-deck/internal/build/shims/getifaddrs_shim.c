/*
 * LD_PRELOAD shim: stub out getifaddrs/freeifaddrs.
 *
 * OpenShell's supervisor blocks AF_NETLINK via seccomp. Node.js (libuv)
 * calls getifaddrs(), which internally opens an AF_NETLINK socket to
 * enumerate interfaces. The blocked syscall surfaces as an opaque
 * "getifaddrs returned an error" crash in Claude Code.
 *
 * This shim intercepts getifaddrs at the glibc level and returns a
 * synthetic loopback entry. The real syscall is never attempted.
 *
 * Compile: gcc -shared -fPIC -o getifaddrs_shim.so getifaddrs_shim.c
 * Use:     LD_PRELOAD=/usr/lib/getifaddrs_shim.so claude
 */

#include <ifaddrs.h>
#include <net/if.h>
#include <netinet/in.h>

static struct sockaddr_in lo_addr = {
    .sin_family = AF_INET,
    .sin_addr.s_addr = 0x0100007f /* 127.0.0.1 in network byte order */
};

static struct sockaddr_in lo_mask = {
    .sin_family = AF_INET,
    .sin_addr.s_addr = 0xffffffff /* 255.255.255.255 */
};

static struct ifaddrs lo_entry = {
    .ifa_next  = 0,
    .ifa_name  = "lo",
    .ifa_flags = IFF_UP | IFF_LOOPBACK | IFF_RUNNING,
    .ifa_addr  = (struct sockaddr *)&lo_addr,
    .ifa_netmask = (struct sockaddr *)&lo_mask,
    .ifa_broadaddr = 0,
    .ifa_data  = 0
};

int getifaddrs(struct ifaddrs **ifap) {
    *ifap = &lo_entry;
    return 0;
}

void freeifaddrs(struct ifaddrs *ifa) {
    (void)ifa;
}
