#!/bin/bash
echo "scanning for dependency updates..."
# List direct dependencies with updates available
go list -u -m -f '{{if .Update}}{{.Path}}: {{.Version}} -> {{.Update.Version}}{{end}}' all 2>/dev/null
echo "scan complete."