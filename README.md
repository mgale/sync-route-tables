# sync-route-tables
Sync kernel route table changes from the main to a custom route table.

# Help
```
SYNOPSIS:
    sync-route-tables --managed-rt <int> [--all-bridges] [--help|-?]
                      [--version|-V] [<args>]

REQUIRED PARAMETERS:
    --managed-rt <int>    Route table to push changes into

OPTIONS:
    --all-bridges         Sync all changes, otherwised only docker networks are synced (default: false)

    --help|-?             (default: false)

    --version|-V          (default: false)


```

# Output
```
2022/02/14 22:50:30 starting ...
2022/02/14 22:50:30 ##########################################
2022/02/14 22:50:30 Getting docker networks ...
2022/02/14 22:50:30 Skipping default docker bridge
2022/02/14 22:50:30 Adding network info for: bridge
2022/02/14 22:50:30 Adding network info for: orchestrator_default
2022/02/14 22:50:30 Adding network info for: dev_admin_setup_credentials_network
2022/02/14 22:50:30 Completed getting docker networks
2022/02/14 22:50:30 ##########################################
2022/02/14 22:50:30 Syncing all routes over ...
2022/02/14 22:50:30 Routes on: br-a416868123e4
2022/02/14 22:50:30 Adding new route: Name: eth1, DestIP: 10.169.32.0/20
2022/02/14 22:50:30 Adding new route: Name: docker0, DestIP: 10.253.255.0/26
2022/02/14 22:50:30 Adding new route: Name: eth0, DestIP: 169.254.169.254/32
2022/02/14 22:50:30 Adding new route: Name: br-f4ac045dbc5d, DestIP: 169.254.170.0/24
2022/02/14 22:50:30 Adding new route: Name: br-b8c206d542bc, DestIP: 172.21.0.0/16
2022/02/14 22:50:30 Adding new route: Name: eth0, DestIP: 198.19.64.0/18
2022/02/14 22:50:30 Routes on: br-b8c206d542bc
2022/02/14 22:50:30 Adding new route: Name: br-b8c206d542bc, DestIP: 172.21.0.0/16
2022/02/14 22:50:30 Routes on: br-f4ac045dbc5d
2022/02/14 22:50:30 Adding new route: Name: br-f4ac045dbc5d, DestIP: 169.254.170.0/24
2022/02/14 22:50:30 Listening for route updates ...

```