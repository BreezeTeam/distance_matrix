// Package lbsyun_baidu_com
// @Author Euraxluo  16:59:00
package api_map_baidu_com

type LBSYunOption struct {
	batch  int    // 途径点个数 默认为10，且最大为
	ak     string // 开发者密钥
	origin string //起点

	mode        int    // mode 出行方式
	destination string //终点
	waypoints   string //途经点

	// tactics
	//默认值：0。
	//可选值：
	//0：常规路线，即多数用户常走的一条经验路线，满足大多数场景需求，是较推荐的一个策略
	//1：不走高速
	//2：躲避拥堵
	//3：距离较短
	tactics int //路线偏好

	// coord_type
	//默认bd09ll
	//允许的值为：
	//bd09ll：百度经纬度坐标
	//bd09mc：百度墨卡托坐标
	//gcj02：国测局加密坐标
	//wgs84：gps设备获取的坐标
	coord string //输入坐标类型

	// ret_coordtype
	//bd09ll：百度经纬度坐标
	//gcj02：国测局加密坐标
	ret_coordtype string //返回的坐标类型
}
