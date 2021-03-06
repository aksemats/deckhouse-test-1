# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

bb-event-on 'resolv-copier-service-changed' '_on_resolv_copier_service_changed'
function _on_resolv_copier_service_changed() {
{{- if ne .runType "ImageBuilding" }}
    systemctl daemon-reload
    systemctl restart resolv-copier.service
{{- end }}
    systemctl enable resolv-copier.service
}

mkdir -p /var/lib/bashible/resolv

bb-sync-file /usr/local/bin/d8-resolv-copier - << "EOF"
#!/bin/bash
set -e

resolv_dir="/var/lib/bashible/resolv"

# Detect systemd-resolved
if grep -q '^nameserver 127.0.0.53' /etc/resolv.conf ; then
  resolv_conf_path="/run/systemd/resolve/resolv.conf"
else
  resolv_conf_path="/etc/resolv.conf"
fi
cp -f $resolv_conf_path $resolv_dir/resolv.conf
while inotifywait -qq -e modify,delete_self $resolv_conf_path; do
  cp -f $resolv_conf_path $resolv_dir/resolv.tmp
  if ! cmp -s $resolv_dir/resolv.tmp $resolv_dir/resolv.conf; then
    cat $resolv_dir/resolv.tmp > $resolv_dir/resolv.conf
  fi
  rm -f $resolv_dir/resolv.tmp
done
EOF
chmod +x /usr/local/bin/d8-resolv-copier

bb-sync-file /etc/systemd/system/resolv-copier.service - resolv-copier-service-changed << "EOF"
[Unit]
Description=Resolv Copier
After=network.target
Before=kubelet.service

[Service]
Type=simple
User=root
Restart=on-failure
RestartSec=5
KillMode=process
ExecStart=/usr/local/bin/d8-resolv-copier

[Install]
WantedBy=multi-user.target
EOF
