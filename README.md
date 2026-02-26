# CollieOS 🐕

> **人指导牧羊犬，牧羊犬管理羊群。**
> *Humans guide the collie; the collie manages the flock.*

---

## 概述 / Overview

**CollieOS** 是一个以 AI 为核心的人机协作框架。它的灵感来源于牧羊犬（Collie）管理羊群的方式：

- 🧑‍💼 **人（Human）** — 发出高层指令，指导牧羊犬的行为目标
- 🐕 **牧羊犬 / Collie（AI Agent）** — 理解人的意图，自主调度和管理任务
- 🐑 **羊群 / Flock（Tasks & Sub-agents）** — 被牧羊犬驱动和协调完成的具体任务或子智能体

**CollieOS** is an AI-centric human-machine collaboration framework inspired by the way a Border Collie herds sheep:

- 🧑‍💼 **Human** — Provides high-level goals and guidance
- 🐕 **Collie (AI Agent)** — Understands human intent and autonomously orchestrates work
- 🐑 **Flock (Tasks & Sub-agents)** — The concrete tasks or sub-agents driven and coordinated by the Collie

---

## 核心理念 / Core Concept

```
  Human  ──── (goals & constraints) ────▶  Collie AI
                                               │
                          ┌────────────────────┼─────────────────────┐
                          ▼                    ▼                     ▼
                       Task A              Task B               Sub-agent C
                      (sheep)             (sheep)               (sheep)
```

人只需要告诉牧羊犬"把羊群赶到山那边"，牧羊犬负责规划路径、协调每只羊的行动，最终完成目标。人无需关心每一个细节，只需在关键节点给予反馈和纠正。

Humans simply tell the Collie *where* the flock needs to go. The Collie plans the route, coordinates each task, and achieves the goal — without the human needing to micromanage every step. Humans stay in the loop at key checkpoints to provide feedback and correction.

---

## 特性 / Features

- **自主任务编排** — Collie AI 根据目标自动分解、分配并监控任务
- **人在回路（Human-in-the-Loop）** — 人类可以在任何阶段介入、调整方向
- **多智能体协作** — 支持多个子 Agent 并行工作，由 Collie 统一调度
- **可解释的行为** — Collie 的每一步决策对人类透明可追溯

---

- **Autonomous task orchestration** — The Collie AI automatically decomposes, assigns, and monitors tasks toward a goal
- **Human-in-the-Loop** — Humans can intervene and redirect at any stage
- **Multi-agent collaboration** — Multiple sub-agents work in parallel under unified Collie coordination
- **Explainable behavior** — Every Collie decision is transparent and traceable to humans

---

## 快速开始 / Getting Started

> 🚧 项目正在积极开发中，更多文档和示例即将到来。
> 🚧 The project is under active development. More documentation and examples coming soon.

---

## 许可证 / License

本项目遵循 [GNU General Public License v3.0](LICENSE) 开源协议。

This project is licensed under the [GNU General Public License v3.0](LICENSE).