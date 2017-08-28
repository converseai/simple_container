# simple_container
A very simple container services that uses linux namespaces  

Written in GO, uses /bin/ip command to set up network parts, The storge will be in OverlayFS 
Uses a JSON config, 
i.e 
``` 
{

        "id" : "9009",
        "container": {
                "workdir" : "/root",
                "network" : {
                        "bridge":"scontainer",
                        "ip":"10.0.3.5/24",
                        "gateway":"10.0.3.1"
                },
                "rootFs":{
                        "paths" :["/var/lib/images/busybox/", "/var/lib/images/user/"],
                        "ephemeral" : true
                },
                "env":[],
                "exec" : {
                        "command" : "/bin/sh",
                        "args":[-l],
                        "uid":0,
                        "gid":0
                },
                "limits": {
                        "mem":100
                }
        },
        "proxy":{
                  "hostIp":"172.17.1.2",
                  "hostPort":9009,
                  "containerIp":"10.0.3.5",
                  "containerPort":9009
        }
}
```
Proxy will set up a port proxy is needed you can remove that, also if you do not want a new network namespace get rid of the the network namespace.

If you want to set up a bridge where you want to have new networking namespace, following will set it up for you.

``` 
brctl addbr scontainer0
ip link set dev scontainer up
iptables -tnat -N scontainer
iptables -tnat -A PREROUTING -m addrtype --dst-type LOCAL -j scontainer
iptables -tnat -A OUTPUT ! -d 127.0.0.0/8 -m addrtype --dst-type LOCAL -j scontainer
iptables -tnat -A POSTROUTING -s 10.0.3.0/24 ! -o scontainer0 -j MASQUERADE
iptables -tnat -A scontainer -i scontainer0 -j RETURN
ip addr add 10.0.3.1/24 dev scontainer0
iptables -A FORWARD -i eth0 -o scontainer0 -j ACCEPT
iptables -A FORWARD -i scontainer0 -o eth0 -j ACCEPT
```

You will need to enable ipforwading you can do this by doing 
```
echo 1 > /proc/sys/net/ipv4/ip_forward
```
