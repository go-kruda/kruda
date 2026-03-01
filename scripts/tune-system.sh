#!/bin/bash

# TechEmpower system tuning for maximum performance
# Run with: sudo ./tune-system.sh

set -e

echo "Applying TechEmpower system tuning..."

# Network tuning
sysctl -w net.core.somaxconn=65535
sysctl -w net.core.netdev_max_backlog=5000
sysctl -w net.core.rmem_max=134217728
sysctl -w net.core.wmem_max=134217728
sysctl -w net.ipv4.tcp_rmem="4096 65536 134217728"
sysctl -w net.ipv4.tcp_wmem="4096 65536 134217728"
sysctl -w net.ipv4.tcp_congestion_control=bbr
sysctl -w net.ipv4.tcp_tw_reuse=1
sysctl -w net.ipv4.tcp_fin_timeout=30
sysctl -w net.ipv4.tcp_keepalive_time=120
sysctl -w net.ipv4.tcp_keepalive_probes=3
sysctl -w net.ipv4.tcp_keepalive_intvl=15
sysctl -w net.ipv4.tcp_max_syn_backlog=4096
sysctl -w net.ipv4.ip_local_port_range="1024 65535"

# File descriptor limits
echo "* soft nofile 1000000" >> /etc/security/limits.conf
echo "* hard nofile 1000000" >> /etc/security/limits.conf
echo "root soft nofile 1000000" >> /etc/security/limits.conf
echo "root hard nofile 1000000" >> /etc/security/limits.conf

# Memory and CPU tuning
sysctl -w vm.swappiness=1
sysctl -w kernel.sched_migration_cost_ns=5000000

# Make changes persistent
cat >> /etc/sysctl.conf << EOF
# TechEmpower optimizations
net.core.somaxconn=65535
net.core.netdev_max_backlog=5000
net.core.rmem_max=134217728
net.core.wmem_max=134217728
net.ipv4.tcp_rmem=4096 65536 134217728
net.ipv4.tcp_wmem=4096 65536 134217728
net.ipv4.tcp_congestion_control=bbr
net.ipv4.tcp_tw_reuse=1
net.ipv4.tcp_fin_timeout=30
net.ipv4.tcp_keepalive_time=120
net.ipv4.tcp_keepalive_probes=3
net.ipv4.tcp_keepalive_intvl=15
net.ipv4.tcp_max_syn_backlog=4096
net.ipv4.ip_local_port_range=1024 65535
vm.swappiness=1
kernel.sched_migration_cost_ns=5000000
EOF

echo "System tuning applied. Reboot recommended for full effect."