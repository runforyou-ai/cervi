package brand

import "testing"

// TestNameMatchesLocale 验证产品名称先精确匹配语言标签，再匹配主语言，最后回退到默认语言。
func TestNameMatchesLocale(t *testing.T) {
	value := Brand{Names: map[string]string{"en-US": "Acme", "zh-CN": "艾克米"}}
	cases := map[string]string{"zh-CN": "艾克米", "zh-TW": "艾克米", "zh": "艾克米", "en-GB": "Acme", "fr-FR": "Acme", "": "Acme"}
	for locale, want := range cases {
		if got := value.Name(locale); got != want {
			t.Errorf("Name(%q) = %q，期望 %q", locale, got, want)
		}
	}
}

// TestBuildBrandIsValid 验证随程序嵌入的构建品牌有效。
func TestBuildBrandIsValid(t *testing.T) {
	if err := Build().Validate(); err != nil {
		t.Fatalf("构建品牌无效: %v", err)
	}
}

// TestConfigureAppliesOverride 验证部署覆盖只替换给出的字段，无效覆盖不改变当前品牌。
func TestConfigureAppliesOverride(t *testing.T) {
	t.Cleanup(func() { current.Store(nil) })
	if err := Configure(Override{Names: map[string]string{"zh-CN": "艾克米"}, SDKName: "Acme"}); err != nil {
		t.Fatalf("应用覆盖失败: %v", err)
	}
	value := Current()
	if value.Name("zh-CN") != "艾克米" || value.Name("en-US") != Build().Name("en-US") || value.SDKName != "Acme" || value.Slug != Build().Slug {
		t.Fatalf("覆盖后的品牌 = %#v", value)
	}
	if err := Configure(Override{SDKName: "acme-desk"}); err == nil {
		t.Fatal("应拒绝无效的嵌入脚本对象名")
	}
	if Current().SDKName != "Acme" {
		t.Fatal("无效覆盖不应改变当前品牌")
	}
}
