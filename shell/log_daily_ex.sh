#!/bin/bash
#
# 日志统一清理shell
# 2017.11.2
#------------------------------------配置区  开始---------------------------------------
#1.业务日志直接进行日志删除操作：serviceRmLogPathList
#param 1  源文件目录
#param 2  保留天数，+7(保留7天内) +3(保留3天内)
#param 3  源文件(支持正则)

serviceRmLogPathList=(
'/data/www/cow_battle_prod1/battle/logs/ +3 *.log.*'
'/data/www/cow_battle_prod1/console/runtime/logs/ +3 *.log.*'
'/data/www/cow_battle_prod2/battle/logs/ +3 *.log.*'
'/data/www/cow_battle_prod2/console/runtime/logs/ +3 *.log.*'
'/data/log_cow/ +7 *.log.*'
'/data/log_cow2/ +7 *.log.*'
)

#2.业务日志，进行备份操作：serviceBakLogPathList
#param $1  源文件目录
#param $2  目标文件目录
#param $3  源文件
#param $4  目标用户组
serviceYesterday=$(date "+%Y%m%d" --date="-1 day")
serviceOwerGroup='xiaohui:xiaohui'

serviceBakLogPathList=(
"/data/www/cow/cow/runtime/logs/ /data/log_cow/ *log.${serviceYesterday} ${serviceOwerGroup}"
"/data/www/cow2/cow/runtime/logs/ /data/log_cow2/ *log.${serviceYesterday} ${serviceOwerGroup}"
)

#3.系统日志，进行备份操作：sysBakLogPathList
#param $1  源文件目录
#param $2  目标文件目录
#param $3  源文件
#param $4  目标文件
#param $5  目标用户组
sysYesterday=$(date -d "yesterday" +%Y-%m-%d)
sysOwerGroup='xiaohui:xiaohui'

sysBakLogPathList=(
"/home/wwwlogs/ /data/log_knowbox/ access access_${sysYesterday} ${sysOwerGroup}"
"/home/wwwlogs/ /data/log_knowbox/ access2 access2_${sysYesterday} ${sysOwerGroup}"
"/home/wwwlogs/ /data/log_knowbox/ nginx_error nginx_error_${sysYesterday} ${sysOwerGroup}"
"/home/wwwlogs/ /data/log_knowbox/ php-fpm php-fpm_${sysYesterday} ${sysOwerGroup}"
"/home/wwwlogs/ /data/log_knowbox/ php-slow php-slow_${sysYesterday} ${sysOwerGroup}"
"/home/wwwlogs/ /data/log_knowbox/ redis redis_${sysYesterday} ${sysOwerGroup}"
)

#------------------------------------配置区  结束---------------------------------------


function service_log_rm()
{
    local path=$1
    local mtime=$2
    local prefix=$3
    if [ -d ${path} ]; then
        find ${path} -type f -name ${prefix} -mtime ${mtime} 2>/dev/null -exec rm -f {} \;
    fi
}

function service_log_bak()
{
    local src=$1
    local des=$2
    local prefix=$3
    local ower_group=$4
    if [ -d ${src} ]; then
        if [ ! -z "$(ls ${src}${prefix} 2>/dev/null)" ]; then
            chown ${ower_group} ${src}${prefix}
            if [ ! -d $des ]; then
                mkdir -p ${des} && chmod -R 777 ${des}
            fi
            mv ${src}${prefix} ${des}
        fi
    fi
}

function sys_log_bak()
{
    local src=$1
    local des=$2
    local origin_name=$3
    local after_name=$4
    local ower_group=$5
    if [ -d ${src} ]; then
        if [ -f ${src}${origin_name} ]; then
            chown ${ower_group} ${src}${origin_name}
            if [ ! -d ${des} ]; then
                mkdir -p ${des} && chmod -R 777 ${des}
            fi
            mv ${src}${origin_name} ${des}${after_name}
            nginx_process ${origin_name}
        fi
    fi
}

function nginx_process()
{
    local filename=$1
    if [ $filename == 'access' -o $filename == 'nginx_error' ]; then
        ## 向 Nginx 主进程发送 USR1 信号。USR1 信号是重新打开日志文件
       kill -USR1 $(cat /usr/local/nginx/logs/nginx.pid)
    fi
}


for one in "${serviceRmLogPathList[@]}"
do
    service_log_rm ${one}
done


for one in "${serviceBakLogPathList[@]}"
do
    service_log_bak ${one}
done

for one in "${sysBakLogPathList[@]}"
do
    sys_log_bak ${one}
done

