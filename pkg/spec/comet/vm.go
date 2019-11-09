// Copyright 2019 <chaishushan{AT}gmail.com>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package comet

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
)

const (
	PC_MAX   = 0xFC00 // PC最大地址
	SP_START = 0xFC00 // SP栈开始地址
)

type VM struct {
	PC       int                   // 指令计数器
	FR       int16                 // 标志寄存器
	GR       [5]int16              // 通用寄存器
	Mem      [1 << 16]int16        // 64KB内存
	RW       io.ReadWriter         // 标准输入输出(VM自身使用)
	Shutdown bool                  // 已经关机
	Syscall  func(ctx *VM, id int) // 系统调用(有部分必须要实现的保留编号)
}

type stdReadWriter struct{}

func (*stdReadWriter) Read(p []byte) (n int, err error) {
	return os.Stdin.Read(p)
}
func (*stdReadWriter) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

func New(rw io.ReadWriter, prog []int16, pc int) *VM {
	p := &VM{PC: pc, RW: rw}
	copy(p.Mem[:], prog)

	if p.RW == nil {
		p.RW = new(stdReadWriter)
	}

	return p
}

func (p *VM) Run() {
	if p.Shutdown {
		return
	}
	for !p.Shutdown {
		p.StepRun()
	}
}

func (p *VM) StepRun() {
	if p.Shutdown {
		return
	}

	var op = p.Mem[p.PC] / 0x100
	var gr = p.Mem[p.PC] % 0x100 / 0x10
	var xr = p.Mem[p.PC] % 0x10
	var adr = p.Mem[p.PC+1]

	if gr < 0 || gr > 4 {
		fmt.Printf("非法指令：mem[%x] = %x\n", p.PC, p.Mem[p.PC])
		p.Shutdown = true
		return
	}
	if xr < 0 || xr > 4 {
		fmt.Printf("非法指令：mem[%x] = %x\n", p.PC, p.Mem[p.PC])
		p.Shutdown = true
		return
	}
	if xr != 0 {
		adr += p.GR[xr]
	}

	// 指令解码
	switch op {
	case HALT:
		p.PC += 1
		p.Shutdown = true
	case LD:
		p.PC += 2
		p.GR[gr] = p.Mem[adr]
	case ST:
		p.PC += 2
		p.Mem[adr] = p.GR[gr]
	case LEA:
		p.PC += 2
		p.GR[gr] = adr
		p.FR = p.GR[gr]
	case ADD:
		p.PC += 2
		p.GR[gr] += p.Mem[adr]
		p.FR = p.GR[gr]
	case SUB:
		p.PC += 2
		p.GR[gr] -= p.Mem[adr]
		p.FR = p.GR[gr]
	case MUL:
		p.PC += 2
		p.GR[gr] *= p.Mem[adr]
		p.FR = p.GR[gr]
	case DIV:
		p.PC += 2
		p.GR[gr] /= p.Mem[adr]
		p.FR = p.GR[gr]
	case MOD:
		p.PC += 2
		p.GR[gr] %= p.Mem[adr]
		p.FR = p.GR[gr]
	case AND:
		p.PC += 2
		p.GR[gr] &= p.Mem[adr]
		p.FR = p.GR[gr]
	case OR:
		p.PC += 2
		p.GR[gr] |= p.Mem[adr]
		p.FR = p.GR[gr]
	case EOR:
		p.PC += 2
		p.GR[gr] ^= p.Mem[adr]
		p.FR = p.GR[gr]
	case SLA:
		p.PC += 2
		p.GR[gr] <<= p.Mem[adr]
		p.FR = p.GR[gr]
	case SRA:
		p.PC += 2
		p.GR[gr] >>= p.Mem[adr]
		p.FR = p.GR[gr]
	case SLL:
		p.PC += 2
		p.GR[gr] = int16(uint16(p.GR[gr]) << p.Mem[adr])
		p.FR = p.GR[gr]
	case SRL:
		p.PC += 2
		p.GR[gr] = int16(uint16(p.GR[gr]) >> p.Mem[adr])
		p.FR = p.GR[gr]
	case CPA:
		p.PC += 2
		p.FR = p.GR[gr] - p.Mem[adr]
	case CPL:
		p.PC += 2
		p.FR = int16(uint16(p.GR[gr]) - uint16(p.Mem[adr]))
	case JMP:
		p.PC += 2
		p.PC = int(adr)
	case JPZ:
		p.PC += 2
		if p.FR >= 0 {
			p.PC = int(adr)
		}
	case JMI:
		p.PC += 2
		if p.FR < 0 {
			p.PC = int(adr)
		}
	case JNZ:
		p.PC += 2
		if p.FR != 0 {
			p.PC = int(adr)
		}
	case JZE:
		p.PC += 2
		if p.FR == 0 {
			p.PC = int(adr)
		}
	case PUSH:
		p.PC += 2
		p.Mem[int(p.GR[4]-1)] = p.Mem[adr]
		p.GR[4]--
	case POP:
		p.PC += 1
		p.GR[gr] = p.Mem[p.GR[4]]
		p.GR[4]++
	case CALL:
		p.PC += 2
		p.Mem[p.GR[4]-1] = int16(p.PC)
		p.PC = int(p.Mem[adr])
		p.GR[4]--
	case RET:
		p.PC += 1
		p.PC = int(p.Mem[p.GR[4]])
		p.GR[4]++

	case SYSCALL:
		if p.Syscall != nil {
			id := p.Mem[p.PC] % 0x100
			p.Syscall(p, int(id))
		}

	default:
		p.Shutdown = true
		fmt.Printf("非法指令：mem[%x] = %x\n", p.PC, p.Mem[p.PC])
	}
}

func (p *VM) DebugRun() {
	var (
		backup  = *p
		stepcnt int
		pntflag bool
		traflag bool
	)

	fmt.Println("调试 （帮助输入 help）...")
	fmt.Println()

	for !p.Shutdown {
		fmt.Print("输入命令: ")

		bf := bufio.NewReader(p.RW)
		line, _, _ := bf.ReadLine()

		var cmd, x1, x2 = "", 0, 0
		n, _ := fmt.Fscanf(bytes.NewBuffer(line), "%s%d%d", &cmd, &x1, &x2)

		switch cmd {
		case "help", "h":
			fmt.Println(p.DebugHelp())
		case "go", "g":
			stepcnt = 0
			for !p.Shutdown {
				stepcnt++
				if traflag {
					fmt.Println(p.InsString(p.PC, 1))
				}

				// 单步执行(可能执行HALT关机指令)
				p.StepRun()
			}
			if pntflag {
				fmt.Printf("执行指令数目 = %d\n", stepcnt)
			}

		case "step", "s":
			if n >= 2 {
				stepcnt = x1
			} else {
				stepcnt = 1
			}

			var i int
			for i = 0; i < stepcnt && !p.Shutdown; i++ {
				if traflag {
					fmt.Println(p.InsString(p.PC, 1))
				}

				// 单步执行(可能执行HALT关机指令)
				p.StepRun()
			}
			if pntflag {
				fmt.Printf("执行指令数目 = %d\n", i)
			}

		case "jump", "j":
			if n >= 2 {
				fmt.Printf("指令跳转到 %x\n", x1)
				p.PC = x1
			} else {
				fmt.Println("错误: 缺少跳转地址")
			}

		case "regs", "r":
			fmt.Println("显示寄存器数据")

			switch {
			case p.FR > 0:
				fmt.Printf("GR[0] = %4x\tPC = %4x\n", p.GR[0], p.PC)
				fmt.Printf("GR[1] = %4x\tSP = %4x\n", p.GR[1], p.GR[4])
				fmt.Printf("GR[2] = %4x\tFR =   00\n", p.GR[2])
				fmt.Printf("GR[3] = %4x\n", p.GR[3])
			case p.FR < 0:
				fmt.Printf("GR[0] = %4x\tPC = %4x\n", p.GR[0], p.PC)
				fmt.Printf("GR[1] = %4x\tSP = %4x\n", p.GR[1], p.GR[4])
				fmt.Printf("GR[2] = %4x\tFR =   10\n", p.GR[2])
				fmt.Printf("GR[3] = %4x\n", p.GR[3])
			default:
				fmt.Printf("GR[0] = %4x\tPC = %4x\n", p.GR[0], p.PC)
				fmt.Printf("GR[1] = %4x\tSP = %4x\n", p.GR[1], p.GR[4])
				fmt.Printf("GR[2] = %4x\tFR =   01\n", p.GR[2])
				fmt.Printf("GR[3] = %4x\n", p.GR[3])
			}

		case "iMem", "imem", "i":
			fmt.Println("显示内存指令")

			if n < 2 {
				x1 = p.PC
			}
			if n < 3 {
				x2 = 1
			}

			fmt.Println(p.InsString(x1, x2))

		case "dMem", "dmem", "d":
			if n < 2 {
				x1 = p.PC
			}
			if n < 3 {
				x2 = 1
			}

			for i := 0; i < x2 && i < len(p.Mem); i++ {
				fmt.Printf("mem[%-4x] = %x\n", x1, p.Mem[x1])
				x1++
			}

		case "alter", "a":
			if n == 3 {
				fmt.Printf("修改内存数据  mem[%x] = %x\n", x1, x2)
				p.Mem[x1] = int16(x2)
			} else {
				fmt.Println("修改内存数据 失败！")
			}

		case "trace", "t":
			traflag = !traflag
			if traflag {
				fmt.Println("指令显示功能 打开")
			} else {
				fmt.Println("指令显示功能 关闭")
			}

		case "print", "p":
			pntflag = !pntflag
			if pntflag {
				fmt.Println("指令计数功能 打开")
			} else {
				fmt.Println("指令计数功能 关闭")
			}

		case "clear", "c":
			fmt.Println("程序重新载入内存")
			*p = backup
			stepcnt = 0

		case "quit", "q":
			fmt.Println("退出调试...")
			return

		default:
			fmt.Println("未知命令", cmd)
		}
	}
}

func (p *VM) DebugHelp() string {
	return `命令列表:
  h)elp           显示本命令列表
  g)o             运行程序直到停止
  s)tep  <n>      执行 n 条指令 （默认为 1 ）
  j)ump  <b>      跳转到 b 地址 （默认为当前地址）
  r)egs           显示寄存器内容
  i)Mem  <b <n>>  显示从 b 开始 n 个内存数据
  d)Mem  <b <n>>  显示从 b 开始 n 个内存指令
  a(lter <b <v>>  修改 b 位置的内存数据为 v 值
  t)race          开关指令显示功能
  p)rint          开关指令计数功能
  c)lear          重置模拟器内容
  q)uit           终止模拟器
`
}

func (p *VM) InsString(pc, n int) string {
	var buf bytes.Buffer
	for i := 0; i < n; i++ {
		var (
			op        = p.Mem[pc] / 0x100
			gr        = p.Mem[pc] % 0x100 / 0x10
			xr        = p.Mem[pc] % 0x10
			adr       = p.Mem[pc+1]
			syscallId = p.Mem[pc] % 0x100
		)

		if op > RET {
			fmt.Fprintf(&buf, "mem[%-4x]: 未知\n", pc)
			break
		}
		if gr < 0 || gr > 4 {
			fmt.Fprintf(&buf, "mem[%-4x]: 未知\n", pc)
			break
		}

		switch {
		case op == SYSCALL:
			fmt.Fprintln(&buf, "syscall", syscallId)
			pc += 1
		case op == HALT || op == RET:
			fmt.Fprintln(&buf)
			pc += 1
		case op == POP:
			fmt.Fprintf(&buf, "GR%d\n", gr)
			pc += 1
		case op < CPL:
			fmt.Fprintf(&buf, "GR%d, %x", gr, adr)
			if xr != 0 {
				fmt.Fprintf(&buf, ", GR%d", xr)
			}
			fmt.Println()
			pc += 2
		default:
			fmt.Fprintf(&buf, "%x", adr)
			if xr != 0 {
				fmt.Fprintf(&buf, ", GR%d", xr)
			}
			fmt.Println()
			pc += 2
		}
	}

	return buf.String()
}