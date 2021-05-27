#!/bin/bash
#
# date: 2020.1.13
# usage: bash deploy.sh 10.216.2.30
#
# still need passord in scp & ssh command
# can optimize by expect command
################config#########################

onlineIP=(
10.225.2.114
10.225.1.84
10.225.3.77
10.225.3.168
10.225.0.72
10.225.3.236
10.225.2.112
)

for ip in ${onlineIP[@]}
do
{
	echo "${ip} begin execute..."
	echo "bash deploy.sh ${ip}"
	bash deploy.sh ${ip}
} &
done

wait

echo '全部执行结束'
