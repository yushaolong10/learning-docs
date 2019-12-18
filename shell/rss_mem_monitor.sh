#!/bin/bash
# date: 2019/12/18
# desc: monitor linux process rss memory.
# usage:
#	bash rss_mem_monitor.sh ${pid} ${output_file}
# examples:
# 	bash rss_mem_monitor.sh  1810  mem_1810
#

pid=$1
file=$2

echo "pid:[${pid}] begin monitor rss memory..."
while true
do
	if [ ! -e /proc/$pid/status ]; then
		echo "pid:[${pid}] error. process not exist"
		exit
	fi
	date +"%Y-%m-%d %H:%M:%S" >> $file
	cat /proc/$pid/status | grep VmRSS >> $file
	sleep 30s #every 30s output
done
