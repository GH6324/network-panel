"""Both subscription UIs expose the personalized Shadowrocket profile."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]
RULE_URL = (
    "https://raw.githubusercontent.com/LingJingMaster/Shadowrocket-Rules/"
    "refs/heads/main/Shadowrocket.conf"
)


class ShadowrocketRuleGuidanceTest(unittest.TestCase):
    def test_both_frontends_link_rules_and_explain_activation(self):
        for frontend in ("vite-frontend", "vite-frontend-v2"):
            with self.subTest(frontend=frontend):
                page = (ROOT / frontend / "src/pages/subscription.tsx").read_text()
                self.assertIn(RULE_URL, page)
                self.assertIn("配置", page)
                self.assertIn("设为使用中", page)
                self.assertIn("节点订阅", page)
                self.assertIn("仅包含节点", page)
                self.assertIn("HK/JP/US", page)
                self.assertIn("/api/v1/subscription/shadowrocket?rules=1&token=", page)
                self.assertIn("/api/v1/subscription/shadowrocket?token=", page)
                self.assertIn("它已包含节点和规则", page)

    def test_node_subscription_stays_a_uri_list(self):
        backend = (ROOT / "golang-backend/internal/app/controller/subscription.go").read_text()
        function = backend.split("func buildShadowrocket(", 1)[1].split("func buildSurgeConfig(", 1)[0]
        self.assertIn('strings.Join(lines, "\\n")', function)
        self.assertNotIn("[Rule]", function)


if __name__ == "__main__":
    unittest.main()
