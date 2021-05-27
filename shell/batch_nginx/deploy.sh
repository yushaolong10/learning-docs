#!/bin/bash
#
# date: 2020.1.13
# usage: bash deploy.sh 10.216.2.30
#
# still need passord in scp & ssh command
# can optimize by expect command
################config#########################
remote_ip=$1
user="yushaolong"
pwd="yourpassword"

#########################################

sudoroot="echo ${pwd} | sudo -S "

ssh ${user}@${remote_ip} >./remote.${remote_ip}.log 2>&1 << EOF

${sudoroot} mkdir -p /data/nginx/logs 
${sudoroot} chmod 777 /data/nginx/logs
${sudoroot} mkdir -p /data/log/nginx 
${sudoroot} chmod 777 /data/log/nginx


#2.start
${sudoroot} /usr/local/nginx/sbin/nginx

echo 'nginx start success'
exit
EOF
echo "remote done success!"
