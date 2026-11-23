package securefield

import "testing"

func TestRoundTrip(t *testing.T) {
	box, err := New("test-passphrase")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cases := []string{"", "1234", "#门禁 8888", "钥匙放在电表箱左侧，蓝色盒子"}
	for _, plain := range cases {
		enc, err := box.Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plain, err)
		}
		if plain == "" && enc != "" {
			t.Fatalf("空串应保持空串，实际 %q", enc)
		}
		if plain != "" && enc == plain {
			t.Fatalf("密文不应等于明文：%q", enc)
		}
		got, err := box.Decrypt(enc)
		if err != nil {
			t.Fatalf("Decrypt(%q): %v", enc, err)
		}
		if got != plain {
			t.Fatalf("往返不一致：%q → %q", plain, got)
		}
	}
}

// TestNonceRandomized 同一明文两次加密必须不同，
// 否则攻击者能靠密文比对判断两户用了同一个门禁密码。
func TestNonceRandomized(t *testing.T) {
	box, _ := New("test-passphrase")
	a, err := box.Encrypt("8888")
	if err != nil {
		t.Fatal(err)
	}
	b, err := box.Encrypt("8888")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("同一明文的两次密文相同，nonce 未随机化")
	}
}

func TestWrongKeyFails(t *testing.T) {
	a, _ := New("key-a")
	b, _ := New("key-b")
	enc, err := a.Encrypt("8888")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Decrypt(enc); err == nil {
		t.Fatal("换密钥后应解不开，而不是返回错误明文")
	}
}

func TestTamperedCiphertext(t *testing.T) {
	box, _ := New("test-passphrase")
	enc, _ := box.Encrypt("8888")
	// 改一个字符，GCM 校验应该失败
	if len(enc) < 4 {
		t.Fatal("密文过短")
	}
	bad := enc[:len(enc)-2] + "AA"
	if _, err := box.Decrypt(bad); err == nil {
		t.Fatal("篡改后的密文不应解得开")
	}
}

func TestEmptyPassphraseRejected(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("空口令应报错，避免用无效密钥加密")
	}
}
