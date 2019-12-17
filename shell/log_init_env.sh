#!/bin/bash
#
# 2017.11.2
# 线上日志环境初始化
# 
# 接口机器：
# cowapi1 10.10.116.217
# cowapi2 10.10.44.221
# cowapi3 10.10.114.142
# 对战服务：
# battle-prod2-01 10.10.223.42  gateway/register
# battle-prod2-02 10.10.74.100  worker/robot
# battle-prod2-03 10.10.118.138 worker
# battle-prod1-01 10.10.32.192  gateway/register
# battle-prod1-02 10.10.53.28   worker/robot
# battle-prod1-03 10.10.58.134  worker
# 
# -------------------------------配置  开始---------------------------------------------
onlineIP=(
10.10.116.217
10.10.44.221
10.10.114.142
10.10.223.42
10.10.74.100
10.10.118.138
10.10.32.192
10.10.53.28
10.10.58.134
)

cat >/tmp/crontabList <<STD
1 0 * * * /bin/sh /root/crontab_sh/log_daily_ex.sh >>/root/crontab_sh/log/log_daily_ex.log 2>&1
STD

# -------------------------------配置  结束---------------------------------------------

function linux_expect()
{
    local _command=$1
    local _pwd=$2
    local _send_bash=$3
    local _over_bash=$4
    local _clean_bash=$5
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
	        send \"${_over_bash}\n\"
	    }
	    expect \"*${CMD_PROMPT}*\" { 
	        send \"${_clean_bash}\n\"
	    }
		expect eof
	    "
    } >> /tmp/log_init_env.log 2>&1
}


#检测本机是否安装expect
if [ -z "$(rpm -q 'expect')" ]; then
	read -s -p '本机必须要安装expect扩展，请输入本机登录密码:' LOCALPWD
	echo ${LOCALPWD} | sudo -S yum install expect -y
fi
if [ ! -f $(pwd)/log_daily_ex.sh ]; then
    echo 'log_daily_ex.sh 文件必须在相同的目录'
    exit
fi

#对线上机操作
CMD_PROMPT='~]'
read -p '请输入登录线上机器的用户名 :' USER
read -s -p '请输入登录线上机器的密码 :' PWD
today_time=$(date +%Y%m%d%H%M_%s)
for ip in ${onlineIP[@]}
do
{
	#scp文件
    linux_expect "scp /tmp/crontabList $(pwd)/log_daily_ex.sh ${USER}@${ip}:/tmp/" ${PWD} "exit"
	#定时任务添加
	linux_expect "ssh ${USER}@${ip}" ${PWD} "echo ${PWD} | sudo -S crontab -l >/tmp/crontabList.old.bak.${today_time}" "echo ${PWD} | sudo -S crontab /tmp/crontabList" "exit"
    #日志shell拷贝
    linux_expect "ssh ${USER}@${ip}" ${PWD} "echo ${PWD} | sudo -S chown 'root:root' /tmp/log_daily_ex.sh" "echo ${PWD} | sudo -S mv /tmp/log_daily_ex.sh /root/crontab_sh/" "rm -rf /tmp/crontabList"
} &
done

wait

echo 'crontab 已成功初始化'
