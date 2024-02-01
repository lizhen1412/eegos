package util

import (
	"sync"
)

// Counter 是一个带有互斥锁的计数器结构体
type Counter struct {
	sync.Mutex        // 内嵌互斥锁，用于同步
	Num        uint16 // Num 用于存储计数值
}

// GetNum 是 Counter 的方法，用于获取当前计数值并自增
func (this *Counter) GetNum() uint16 {
	this.Lock()         // 加锁，确保同一时间只有一个协程访问 Num
	defer this.Unlock() // 函数结束时解锁
	this.Num++          // 计数值增加
	num := this.Num     // 获取当前计数值

	return num // 返回当前计数值
}
