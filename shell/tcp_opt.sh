#!/bin/bash
#
# 调优linux tcp参数
#
# 2020.01.15
#
# yushaolong@360.cn
#
# -------------------------------配置  开始---------------------------------------------

#添加目标机器的ip列表
onlineIP=(
10.21.2.1
10.21.2.2
10.21.2.3
)


#修改执行账户和密码
USER='yourname'
PWD='yourpassword'


#目标文件及日志名称
EXE_FILE='linux_tcp.sh'
LOCAL_LOG='local_expect.log'
REMOTE_EXE_LOG='remote_linux_tcp.log'
sudoroot="echo '${PWD}' | sudo -S "

#生成目标文件
cat > ${EXE_FILE} <<STD
#1.系统文件备份
${sudoroot} cp /etc/sysctl.conf /etc/sysctl.conf.bak.0115
#2.内核参数添加
${sudoroot} echo 'net.ipv4.tcp_syncookies = 1' >> /etc/sysctl.conf
${sudoroot} echo 'net.ipv4.tcp_tw_reuse = 1' >> /etc/sysctl.conf
${sudoroot} echo 'net.ipv4.tcp_tw_recycle = 1' >> /etc/sysctl.conf
${sudoroot} echo 'net.ipv4.tcp_fin_timeout = 30' >> /etc/sysctl.conf
${sudoroot} echo 'net.ipv4.tcp_max_tw_buckets = 15000' >> /etc/sysctl.conf

#3.运行生效
${sudoroot} /sbin/sysctl -p

STD

# -------------------------------配置  结束---------------------------------------------


#对线上机操作
CMD_PROMPT='~]$'

function linux_expect()
{
    local _command=$1
    local _pwd=$2
    local _send_bash=$3
    local _exit_bash=$4
    {
	    expect -c "
	    set timeout 5
	    spawn ${_command}
	    expect {
	        -re \"Are you sure you want to continue connecting(yes/no)?\" {
	            send \"yes\n\"
	            exp_continue
	        }
	        \"*password:\" {
	            send \"${_pwd}\n\"
	            exp_continue
	        }
	        \"*${CMD_PROMPT}*\" { 
	            send \"${_send_bash}\n\"
	        }
	    }
	    expect \"*${CMD_PROMPT}*\" { 
	        send \"${_exit_bash}\n\"
	    }
		expect eof
	    "
    } >> ${LOCAL_LOG} 2>&1
}




for ip in ${onlineIP[@]}
do
{
	echo "${ip} begin execute..."
	#scp目标文件到远程主机
	linux_expect "scp ${EXE_FILE} ${USER}@${ip}:~/" ${PWD} "exit"
	#修改目标文件权限
	linux_expect "ssh ${USER}@${ip}" ${PWD} "echo '${PWD}' | sudo -S chmod +x ~/${EXE_FILE}" "exit"
	#执行目标文件
	linux_expect "ssh ${USER}@${ip}" ${PWD} "echo '${PWD}' | sudo -S bash  ~/${EXE_FILE} > ~/${REMOTE_EXE_LOG}" "exit"
} &
done

wait

echo '全部执行结束'
