#!/bin/bash
##################
# 2018.09.26
#1.服务停止
#2.清理redis
#3.服务启动
##################

#机器列表
#alias aipro1='ssh 10.9.84.249'
#alias aipro2='ssh 10.9.35.193'
#alias aipro3='ssh 10.9.19.51'
#alias aingx1='ssh 10.9.30.35'
#alias aingx2='ssh 10.9.60.198'
#alias aigray='ssh 10.9.123.249'
#

SERVER_LIST=(
'10.9.84.249'
'10.9.35.193'
'10.9.19.51'
)

#redis 配置
REDIS_IP='10.9.189.104'
REDIS_PORT=6379
REDIS_PASSWD='RedisUINr'


#基础参数
TODAY=$(date +'%y%m%d')
BASE_DIR="${HOME}/aiclass_server"
CMD_PROMPT='~]'
STOP_BASH_SCRIPT="${TODAY}.stop-server.sh"
START_BASH_SCRIPT="${TODAY}.start-server.sh"
CLEAR_REDIS_SCRIPT="${TODAY}.clear-redis.sh"

#bash script
STOP_SERVER_COMMAND=$(cat <<EOF
#!/bash/bin\n
#停止服务\n
sudo supervisorctl stop aiclassapi_stable\n
EOF
)
START_SERVER_COMMAND=$(cat <<EOF
#!/bash/bin\n
#启动服务\n
sudo supervisorctl start aiclassapi_stable\n
EOF
)
FLUSH_REDIS_COMMAND=$(cat <<EOF
#!/bash/bin\n
#清除redis\n
redis-cli -h ${REDIS_IP} -p ${REDIS_PORT} -a ${REDIS_PASSWD} -n 0 flushdb\n
EOF
)


function stop_all_server()
{
#输出shell script
echo -e ${STOP_SERVER_COMMAND} >${BASE_DIR}/${STOP_BASH_SCRIPT} 
#执行
for ip in "${SERVER_LIST[@]}"
do
    control_server ${ip} ${STOP_BASH_SCRIPT}
    echo "${ip} 服务已停止"
done
}

function start_all_server()
{
#输出shell script
echo -e ${START_SERVER_COMMAND} >${BASE_DIR}/${START_BASH_SCRIPT} 
for ip in "${SERVER_LIST[@]}"
do
    control_server ${ip} ${START_BASH_SCRIPT}
    echo "${ip} 服务已启动"
done
}

function flush_redis()
{
    echo -e ${FLUSH_REDIS_COMMAND} >${BASE_DIR}/${CLEAR_REDIS_SCRIPT} 
    control_server ${SERVER_LIST[0]} ${CLEAR_REDIS_SCRIPT}
}


function control_server()
{
    local _ip=$1
    local _script_name=$2
	
    linux_expect "scp ${BASE_DIR}/${_script_name} ${_ip}:~/"
    linux_expect "ssh ${_ip}" "bash ~/${_script_name}"
}

function linux_expect()
{
    local _command=$1
    local _send_bash=$2
    {
    expect -c "
    set timeout 60
    spawn ${_command}
    expect {
        -re \"Are you sure you want to continue connecting(yes/no)?\" {
            send \"yes\n\"
            exp_continue
        }
        \"*password:\" {
            send \"\n\"
            exp_continue
        }
        \"*${CMD_PROMPT}*\" { 
            send \"${_send_bash}\n\"
        }
    }
    expect eof
    "
    } >> ${BASE_DIR}/aiclass.server.${TODAY}.run.log 2>>${BASE_DIR}/aiclass.server.${TODAY}.error.log
}

if [ ! -d ${BASE_DIR} ]; then
    mkdir -p ${BASE_DIR}
fi

date +'%Y-%m-%d %H:%M:%S'
echo -e '\033[43;35m >>>> 开始执行... \033[0m'
echo '#######################'
echo -e '[开始] 停止所有机器服务...'
stop_all_server
echo -e '\033[44;36m[成功]\033[0m 所有服务停止成功!!!'
echo '#######################'
sleep 30
echo '[开始] 清除redis...'
flush_redis
echo -e '\033[44;36m[成功]\033[0m redis清除成功!!!'
echo '#######################'
sleep 30
echo '[开始] 启动所有机器服务...'
start_all_server
echo -e '\033[44;36m[成功]\033[0m 所有服务启动成功!!!'
echo -e '\033[43;35m >>>> 执行结束. \033[0m'
date +'%Y-%m-%d %H:%M:%S'

