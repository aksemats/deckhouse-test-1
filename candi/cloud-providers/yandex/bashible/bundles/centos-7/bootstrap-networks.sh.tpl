#!/bin/bash -e

{{- if hasKey .nodeGroup "instanceClass" }}
  {{- if .nodeGroup.instanceClass.additionalNetworks }}
>&2 echo "ERROR: CentOS support is not implemented yet!"
exit 1
  {{- end }}
{{- end }}