import sys
import tempfile
import unittest
from pathlib import Path

from openpyxl import Workbook, load_workbook

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from scripts.payroll_calculator import calculate, read_rows, write_workbook


class PayrollCalculatorTest(unittest.TestCase):
    def make_book(self, directory, name, headers, rows):
        path = Path(directory) / name
        workbook = Workbook()
        sheet = workbook.active
        sheet.append(headers)
        for row in rows:
            sheet.append(row)
        workbook.save(path)
        return path

    def test_aliases_partial_baseline_and_rules(self):
        with tempfile.TemporaryDirectory() as directory:
            roster = self.make_book(
                directory, "roster.xlsx", ["姓名", "项目", "人员类别", "状态"],
                [["张三", "A项目", "外聘", "正常"], ["李四", "A项目", "外聘", "在册"]],
            )
            history = self.make_book(
                directory, "history.xlsx",
                ["姓名", "项目", "人员类别", "基本工资", "绩效工资", "工龄工资",
                 "职称工资", "施工补贴", "1-3月话补", "降温费", "交通补助", "应发工资"],
                [["张三", "A项目", "外聘", 10000, 3000, 200, 100, 50, 300, 200, 100, 0],
                 ["李四", "A项目", "外聘", 9000, 2000, 0, 0, 40, 200, 0, 0, 0]],
            )
            rows = calculate(
                read_rows(roster), read_rows(history),
                [{"name": "张三", "project": "A项目", "category": "外聘",
                  "actual_work_days": 23, "personal_leave_days": 2}],
                "2026-06", 21.75, 2,
            )
            detail = rows["payroll_detail"][0]
            self.assertEqual(detail["调整后绩效工资"], 3000 - 3000 / 21.75 * 2)
            self.assertEqual(detail["施工补贴"], 0)
            self.assertEqual(detail["加班天数"], 1.25)
            self.assertTrue(any("施工补贴只有月金额" in item["说明"]
                                for item in rows["review_exceptions"]))
            self.assertTrue(any(item["类型"] == "ignored" for item in rows["review_exceptions"]))
            self.assertTrue(any(item.get("姓名") == "张三" for item in rows["review_exceptions"]))

            output = Path(directory) / "result.xlsx"
            write_workbook(rows, output)
            workbook = load_workbook(output, read_only=True)
            try:
                self.assertEqual(
                    workbook.sheetnames,
                    ["payroll_detail", "baseline", "attendance", "reconciliation", "review_exceptions"],
                )
            finally:
                workbook.close()

    def test_missing_workdays_does_not_block_other_items(self):
        rows = calculate(
            [], [{"name": "王五", "project": "B", "category": "外聘",
                  "position_salary": 5000, "performance": 1000}],
            [{"name": "王五", "project": "B", "category": "外聘",
              "actual_work_days": 20}],
            "2026-05", None, None,
        )
        self.assertEqual(rows["payroll_detail"][0]["岗位工资"], 5000)
        self.assertEqual(rows["payroll_detail"][0]["加班费"], 0)
        self.assertTrue(any("基础工作日" in item["说明"] for item in rows["review_exceptions"]))

    def test_construction_total_is_normalized_to_daily_standard(self):
        rows = calculate(
            [],
            [{"name": "赵六", "项目": "C", "category": "外聘",
              "work_days": 26, "position_salary": 5000,
              "performance": 1000, "construction": 780}],
            [{"name": "赵六", "project": "C", "category": "外聘",
              "actual_work_days": 23}],
            "2026-06", 22, 8,
        )
        detail = rows["payroll_detail"][0]
        self.assertEqual(detail["施工补贴"], 690)

    def test_hot_subsidy_is_month_level_and_explicit_overtime_is_counted(self):
        rows = calculate(
            [],
            [{"name": "甲", "项目": "D", "category": "外聘",
              "position_salary": 5000, "performance": 1000,
              "hot": 600, "construction_day": 30},
             {"name": "乙", "项目": "D", "category": "外聘",
              "position_salary": 5000, "performance": 1000,
              "hot": 0, "construction_day": 30}],
            [{"name": "甲", "project": "D", "category": "外聘",
              "actual_work_days": 22, "加班天数": 2},
             {"name": "乙", "project": "D", "category": "外聘",
              "actual_work_days": 22}],
            "2026-06", 22, 8,
        )
        details = {row["姓名"]: row for row in rows["payroll_detail"]}
        baselines = {row["姓名"]: row for row in rows["baseline"]}
        self.assertEqual(baselines["甲"]["高温补贴"], 600)
        self.assertEqual(baselines["乙"]["高温补贴"], 600)
        self.assertEqual(details["甲"]["加班天数"], 2)
        self.assertGreater(details["甲"]["加班费"], 0)

    def test_phone_quarter_total_is_divided_into_monthly_amount(self):
        rows = calculate(
            [],
            [{"name": "钱七", "项目": "E", "category": "外聘",
              "position_salary": 5000, "performance": 1000,
              "phone_1_3": 300}],
            [{"name": "钱七", "project": "E", "category": "外聘",
              "actual_work_days": 22}],
            "2026-03", 22, 0,
        )
        self.assertEqual(rows["payroll_detail"][0]["话费补贴"], 100)

    def test_overtime_counts_attendance_on_single_rest_day(self):
        rows = calculate(
            [],
            [{"name": "孙八", "项目": "F", "category": "外聘",
              "position_salary": 5000, "performance": 1000}],
            [{"name": "孙八", "project": "F", "category": "外聘",
              "actual_work_days": 27,
              "daily_marks": {
                  "2026-06-06": "出勤",  # Saturday is a planned workday for single rest.
                  "2026-06-07": "出勤",  # Both weekend days worked: one overtime day.
              }}],
            "2026-06", 26, 4, "single",
        )
        self.assertEqual(rows["payroll_detail"][0]["加班天数"], 1)

    def test_project_suffix_and_missing_category_still_match_unique_person(self):
        rows = calculate(
            [{"name": "周九", "project": "瀚阅府", "category": "外聘", "status": "正常"}],
            [{"name": "周九", "project": "瀚阅府", "category": "外聘",
              "position_salary": 5000, "performance": 1000,
              "construction_day": 20}],
            [{"name": "周九", "project": "瀚阅府西苑项目",
              "actual_work_days": 20}],
            "2026-06", 22, 8,
        )
        detail = rows["payroll_detail"][0]
        self.assertEqual(detail["岗位工资"], 5000)
        self.assertEqual(detail["施工补贴"], 400)
        self.assertEqual(detail["计算状态"], "已计算")


if __name__ == "__main__":
    unittest.main()
