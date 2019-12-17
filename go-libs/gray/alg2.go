package gray

//ball stand in line, include kind of red and blue ones.
//red instead of which we aim to acquire. blue means not needed
//we begin a test [input param: total equal 100, acquireRed from 0 to 100]
//find that acquire-Red ball not always consistent with queue real-Red ball.
//belong is the result:
//
//if acquire == real { // total 100, match:32
//if acquire-real <= 1 && acquire-real >= -1 { // total 100, match:72
//if acquire-real <= 2 && acquire-real >= -2 { // total 100, match:82
//if acquire-real <= 3 && acquire-real >= -3 { // total 100, match:86
//if acquire-real <= 4 && acquire-real >= -4 { // total 100, match:92
//if acquire-real <= 5 && acquire-real >= -5 { // total 100, match:98
//
//so the algorithm exist some inaccuracy, but it is just ok for us.
func makeBallQueue(total, acquireRed int) (queue []byte) {
	queue = make([]byte, total)
	var colorRed, colorBlue = byte('1'), byte('0')
	//compare and displace
	var mid = int(total / 2)
	if acquireRed > mid {
		colorRed, colorBlue = '0', '1'
		acquireRed = total - acquireRed
	}
	//calculate approximate interval and divisibleRed
	var interval, divisibleRed int
	if acquireRed > 0 {
		interval = int(total / acquireRed)
		divisibleRed = int(total / interval)
	}
	//exceed should eliminate
	var exceed = divisibleRed - acquireRed
	var adjust int
	if exceed > 0 {
		adjust = int(total / exceed)
	}
	//fill queue
	for i := 0; i < total; i++ {
		if interval != 0 && i%interval == 0 {
			queue[i] = colorRed
		} else {
			queue[i] = colorBlue
		}
		if exceed != 0 && i%adjust == 0 {
			queue[i] = colorBlue
		}
	}
	return queue
}
