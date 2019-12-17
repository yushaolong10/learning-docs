package attacker
//usage:
//
//		var innerManagerIns = attacker.NewRepeatManager(RepeatMaxValues, RepeatTTL)
//
//		if innerManagerIns.GetRepeatCount(repeatKey) > 3 { //exceed 3 mean attack
//			//exceed max repeat
//			return true
//		}
//		return false
//
//
//
//benchmark:
//
//
//cpu:4core memory:8g
//maxCount: 1<<20 (round 100w)
//ttl:60s
//===
//benchmark result:
//(1) duration:10000s
//  res mem_used:332m
//  qps_max:698214 qps_min:218107 qps_avg:464030
//(2) duration:20000s
//  res mem_used:332m
//  qps_max:698214 qps_min:218107 qps_avg:467500
//(3) duration:30000s
//  res mem_used:332m
//  qps_max:752970 qps_min:218107 qps_avg:467853
//(4) duration:40000s
//  res mem_used:332m
//  qps_max:752970 qps_min:218107 qps_avg:467323
//(5) duration:50000s
//  res mem_used:332m
//  qps_max:752970 qps_min:218107 qps_avg:463452
//extra:
//min_qps_info => time: 9353s qps: 218107
//max_qps_info => time: 26643s qps: 752970
//res_mem_used is growth from 330m to 400m gradually and gc release to 330m in some condition.