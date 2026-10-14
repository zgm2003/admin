import { useEffect, useMemo, useState } from "react";
import { ArrowRight, Check, Eye, EyeOff, LoaderCircle, LockKeyhole, Mail, Phone, RefreshCw, Sparkles } from "lucide-react";
import { useNavigate } from "react-router-dom";

import type { CaptchaChallenge, LoginType } from "@/api/auth/login";
import { ApiError } from "@/types/http";
import { authStore } from "@/store";
import "./login.scss";

export function LoginPage() {
    const navigate = useNavigate();
    const [config, setConfig] = useState(authStore.state.loginConfig);
    const [loginType, setLoginType] = useState<LoginType>("password");
    const [account, setAccount] = useState("");
    const [password, setPassword] = useState("");
    const [code, setCode] = useState("");
    const [challengeId, setChallengeId] = useState("");
    const [countdown, setCountdown] = useState(0);
    const [captcha, setCaptcha] = useState<CaptchaChallenge | null>(null);
    const [captchaPosition, setCaptchaPosition] = useState(0);
    const [captchaLoading, setCaptchaLoading] = useState(false);
    const [showPassword, setShowPassword] = useState(false);
    const [formError, setFormError] = useState("");
    const [configError, setConfigError] = useState("");
    const [submitting, setSubmitting] = useState(false);

    const activeConfig = config ?? authStore.state.loginConfig;
    const loginOptions = activeConfig?.loginTypes ?? [];
    const activeType = loginOptions.some((item) => item.value === loginType) ? loginType : loginOptions[0]?.value;
    const isCodeLogin = activeType === "email" || activeType === "phone";
    const accountLabel = activeType === "phone" ? "手机号" : "邮箱地址";

    useEffect(() => {
        if (activeConfig !== null) {
            setConfig(activeConfig);
            if (!activeConfig.loginTypes.some((item) => item.value === loginType) && activeConfig.loginTypes[0] !== undefined) {
                setLoginType(activeConfig.loginTypes[0].value);
            }
            return;
        }
        void loadConfig();
    }, [activeConfig, loginType]);

    useEffect(() => {
        if (countdown <= 0) return;
        const timer = window.setInterval(() => setCountdown((value) => Math.max(value - 1, 0)), 1000);
        return () => window.clearInterval(timer);
    }, [countdown]);

    const captchaLeft = useMemo(() => {
        if (captcha === null || captcha.imageWidth <= captcha.tileWidth) return 0;
        const max = captcha.imageWidth - captcha.tileWidth;
        return `${(captchaPosition / max) * 100}%`;
    }, [captcha, captchaPosition]);

    async function loadConfig(): Promise<void> {
        setConfigError("");
        try {
            const next = await authStore.loadLoginConfig();
            setConfig(next);
            const first = next.loginTypes[0];
            if (first !== undefined) setLoginType(first.value);
        } catch {
            setConfigError("登录方式加载失败，请重试");
        }
    }

    function resetMode(nextType: LoginType): void {
        setLoginType(nextType);
        setCode("");
        setChallengeId("");
        setFormError("");
        setCaptcha(null);
        setCountdown(0);
    }

    async function openCaptcha(): Promise<void> {
        if (!isCodeLogin || countdown > 0 || captchaLoading) return;
        if (account.trim() === "") {
            setFormError(`${accountLabel}不能为空`);
            return;
        }
        setFormError("");
        setCaptchaLoading(true);
        try {
            const nextCaptcha = await authStore.api.getCaptcha();
            setCaptcha(nextCaptcha);
            setCaptchaPosition(nextCaptcha.tileX);
        } catch {
            setCaptcha(null);
        } finally {
            setCaptchaLoading(false);
        }
    }

    async function completeCaptcha(): Promise<void> {
        if (captcha === null || !isCodeLogin) return;
        setCaptchaLoading(true);
        try {
            const result = await authStore.api.sendLoginCode(account.trim(), activeType, {
                captchaId: captcha.captchaId,
                captchaAnswer: { x: Math.round(captchaPosition), y: captcha.tileY },
            });
            setChallengeId(result.challengeId);
            setCountdown(result.resendAfterSeconds);
            setCaptcha(null);
        } catch {
            setCaptcha(null);
        } finally {
            setCaptchaLoading(false);
        }
    }

    async function submit(event: React.FormEvent<HTMLFormElement>): Promise<void> {
        event.preventDefault();
        if (activeType === undefined || submitting) return;
        if (account.trim() === "") {
            setFormError(`${accountLabel}不能为空`);
            return;
        }
        if (activeType === "password" && password === "") {
            setFormError("请输入密码");
            return;
        }
        if (isCodeLogin && (code.trim() === "" || challengeId === "")) {
            setFormError("请先获取验证码并填写验证码");
            return;
        }
        setFormError("");
        setSubmitting(true);
        try {
            await authStore.login(activeType === "password" ? { loginType: "password", loginAccount: account.trim(), password } : { loginType: activeType, loginAccount: account.trim(), challengeId, code: code.trim() });
            navigate("/", { replace: true });
        } catch (error) {
            if (error instanceof ApiError && error.code === 10002) setFormError(error.message);
        } finally {
            setSubmitting(false);
        }
    }

    return (
        <main className="auth-shell">
            <section className="auth-story" aria-label="Infinite Canvas">
                <div className="brand-lockup">
                    <span className="brand-mark" aria-hidden="true">
                        <span />
                    </span>
                    <span>infinite canvas</span>
                </div>
                <div className="story-copy">
                    <p className="eyebrow">
                        <Sparkles size={14} /> YOUR VISUAL WORKBENCH
                    </p>
                    <h1>
                        Ideas become
                        <br />
                        <em>visible.</em>
                    </h1>
                    <p>一个为灵感留出空间的无限画布。登录后继续你的创作。</p>
                </div>
                <div className="story-grid" aria-hidden="true">
                    <span />
                    <span />
                    <span />
                    <span />
                    <span />
                    <span />
                </div>
                <p className="story-footer">Create freely. Iterate visibly.</p>
            </section>

            <section className="auth-panel">
                <div className="auth-panel-inner">
                    <div className="mobile-brand">
                        <span className="brand-mark" aria-hidden="true">
                            <span />
                        </span>
                        <span>infinite canvas</span>
                    </div>
                    <div className="auth-heading">
                        <p className="eyebrow">WELCOME BACK</p>
                        <h2>进入你的画布</h2>
                        <p>用账号继续探索、生成和编排。</p>
                    </div>

                    {configError !== "" ? (
                        <div className="inline-error" role="alert">
                            <span>{configError}</span>
                            <button type="button" onClick={() => void loadConfig()}>
                                <RefreshCw size={14} />
                                重试
                            </button>
                        </div>
                    ) : null}

                    {loginOptions.length > 1 ? (
                        <div className="mode-switch" role="tablist" aria-label="登录方式">
                            {loginOptions.map((item) => (
                                <button key={item.value} type="button" role="tab" aria-selected={activeType === item.value} className={activeType === item.value ? "active" : ""} onClick={() => resetMode(item.value)}>
                                    {item.label}
                                </button>
                            ))}
                        </div>
                    ) : null}

                    <form className="auth-form" onSubmit={(event) => void submit(event)}>
                        <label className="field">
                            <span>{accountLabel}</span>
                            <div className="field-control">
                                <span className="field-icon">{activeType === "phone" ? <Phone size={17} /> : <Mail size={17} />}</span>
                                <input
                                    data-testid="login-account"
                                    type={activeType === "phone" ? "tel" : "email"}
                                    value={account}
                                    onChange={(event) => setAccount(event.target.value)}
                                    autoComplete="username"
                                    placeholder={activeType === "phone" ? "输入手机号" : "name@example.com"}
                                />
                            </div>
                        </label>

                        {activeType === "password" ? (
                            <label className="field">
                                <span>密码</span>
                                <div className="field-control">
                                    <span className="field-icon">
                                        <LockKeyhole size={17} />
                                    </span>
                                    <input data-testid="login-password" type={showPassword ? "text" : "password"} value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" placeholder="输入密码" />{" "}
                                    <button type="button" className="field-action" aria-label={showPassword ? "隐藏密码" : "显示密码"} onClick={() => setShowPassword((value) => !value)}>
                                        {showPassword ? <EyeOff size={17} /> : <Eye size={17} />}
                                    </button>
                                </div>
                            </label>
                        ) : isCodeLogin ? (
                            <label className="field">
                                <span>验证码</span>
                                <div className="field-control">
                                    <span className="field-icon">
                                        <Check size={17} />
                                    </span>
                                    <input data-testid="login-code" inputMode="numeric" value={code} onChange={(event) => setCode(event.target.value.replace(/\D/g, "").slice(0, 6))} placeholder="输入 6 位验证码" />{" "}
                                    <button type="button" className="code-action" disabled={countdown > 0 || captchaLoading} onClick={() => void openCaptcha()}>
                                        {countdown > 0 ? `${countdown}s 后重发` : captchaLoading ? "准备中" : "获取验证码"}
                                    </button>
                                </div>
                            </label>
                        ) : (
                            <p className="form-error" role="status">
                                {configError === "" ? "正在读取登录方式…" : "请重试以读取可用登录方式"}
                            </p>
                        )}

                        {formError !== "" ? (
                            <p className="form-error" role="alert">
                                {formError}
                            </p>
                        ) : null}
                        <button className="submit-button" type="submit" disabled={submitting || activeType === undefined || configError !== ""}>
                            {submitting ? <LoaderCircle className="spin" size={17} /> : <ArrowRight size={17} />}
                            {submitting ? "正在进入…" : "进入画布"}
                        </button>
                    </form>

                    <p className="auth-note">账号由平台统一管理。登录即表示你同意继续使用当前工作区。</p>
                </div>
            </section>

            {captcha !== null ? (
                <div className="captcha-backdrop" role="dialog" aria-modal="true" aria-label="完成安全验证">
                    <div className="captcha-card">
                        <div className="captcha-heading">
                            <div>
                                <p className="eyebrow">SECURITY CHECK</p>
                                <h3>拖动滑块完成验证</h3>
                            </div>
                            <button type="button" onClick={() => setCaptcha(null)} aria-label="关闭验证">
                                ×
                            </button>
                        </div>
                        <div className="captcha-image" style={{ aspectRatio: `${captcha.imageWidth} / ${captcha.imageHeight}` }}>
                            <img src={captcha.masterImage} alt="验证码背景" />
                            <img
                                className="captcha-tile"
                                src={captcha.tileImage}
                                alt="验证码拼图"
                                style={{ left: captchaLeft, top: `${(captcha.tileY / captcha.imageHeight) * 100}%`, width: `${(captcha.tileWidth / captcha.imageWidth) * 100}%`, height: `${(captcha.tileHeight / captcha.imageHeight) * 100}%` }}
                            />
                        </div>
                        <input aria-label="验证码滑块" type="range" min="0" max={Math.max(captcha.imageWidth - captcha.tileWidth, 1)} value={captchaPosition} onChange={(event) => setCaptchaPosition(Number(event.target.value))} />
                        <button type="button" className="submit-button" disabled={captchaLoading} onClick={() => void completeCaptcha()}>
                            {captchaLoading ? "验证中…" : "确认位置"}
                        </button>
                    </div>
                </div>
            ) : null}
        </main>
    );
}
