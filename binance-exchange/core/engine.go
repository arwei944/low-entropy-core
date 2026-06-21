// Package core — 撮合引擎 (MatchBox)
//
// 架构说明（符合四原语规范）：
//   - engine_state.go : L1 State — OrderBook 纯内存数据结构（无 I/O）
//   - engine_atom.go  : L1 Atom  — 撮合算法（纯计算）+ MatchEngine 编排器
//
// 原 engine.go 中的代码已拆分到上述两个文件。
package core
