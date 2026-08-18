# POC 能力迁移说明

原始 POC：`E:/workspace/shanjian-poc`。

核心参考模块：

- `app/poc/salary-baseline.ts`：工资基准快照和历史工资解析；
- `app/poc/reference-workbooks.ts`：人员底表解析；
- `app/poc/payroll-calculation.ts`：确定性工资计算；
- `app/poc/payroll-formulas.ts`：公式说明；
- `app/poc/multimodal-attendance.ts`：考勤视觉识别 JSON 结构；
- `app/poc/workbook.ts`：五个 Excel Sheet 的导出结构。

不要迁移 POC 的浏览器 IndexedDB、客户端 PDF 预处理 UI、Cloudflare Worker 或演示页面。Lework 中应将基准和中间产物写入当前项目/任务工作区，识别与计算过程保留来源证据，并使用现有 worker 的文件、模型和 artifact 能力。

POC 的部分核算行为优先于过时设计文档：缺少单个人员基准时允许其他人员继续计算并列出缺失清单；规则不明确的分项必须进入人工复核。
