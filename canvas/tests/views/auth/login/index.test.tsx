import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { LoginPage } from "@/views/auth/login";
import { ApiError, HttpError } from "@/types/http";

const { navigate, store } = vi.hoisted(() => ({
    navigate: vi.fn(),
    store: {
        state: {
            status: "anonymous" as const,
            loginConfig: { loginTypes: [{ value: "password" as const, label: "账号密码" }], allowRegister: false },
        },
        loadLoginConfig: vi.fn().mockResolvedValue({ loginTypes: [{ value: "password", label: "账号密码" }], allowRegister: false }),
        api: { getCaptcha: vi.fn(), sendLoginCode: vi.fn() },
        login: vi.fn().mockResolvedValue({ userId: 1 }),
    },
}));

vi.mock("@/store", () => ({ authStore: store }));
vi.mock("react-router-dom", () => ({ useNavigate: () => navigate }));

describe("LoginPage", () => {
    beforeEach(() => {
        vi.clearAllMocks();
        store.login.mockResolvedValue({ userId: 1 });
    });

    it("renders the server-selected password form without inventing another login mode", async () => {
        render(<LoginPage />);

        expect(await screen.findByTestId("login-password")).toBeInTheDocument();
        expect(screen.queryByRole("tab", { name: "邮箱验证码" })).not.toBeInTheDocument();
    });

    it("submits password credentials to the Canvas auth store and enters the workbench", async () => {
        render(<LoginPage />);
        const account = await screen.findByTestId("login-account");
        fireEvent.change(account, { target: { value: "user@example.com" } });
        fireEvent.change(screen.getByTestId("login-password"), { target: { value: "secret" } });
        fireEvent.click(screen.getByRole("button", { name: "进入画布" }));

        await waitFor(() => expect(store.login).toHaveBeenCalledWith({ loginType: "password", loginAccount: "user@example.com", password: "secret" }));
        expect(navigate).toHaveBeenCalledWith("/", { replace: true });
    });

    it.each([
        { error: new ApiError(10002, "凭据错误", 401), inline: true },
        { error: new HttpError(500, "服务暂时不可用，请稍后重试"), inline: false },
    ])("shows only the credential error inline after login fails ($inline)", async ({ error, inline }) => {
        store.login.mockRejectedValueOnce(error);
        render(<LoginPage />);
        fireEvent.change(await screen.findByTestId("login-account"), { target: { value: "user@example.com" } });
        fireEvent.change(screen.getByTestId("login-password"), { target: { value: "secret" } });
        fireEvent.click(screen.getByRole("button", { name: "进入画布" }));
        await waitFor(() => expect(screen.getByRole("button", { name: "进入画布" })).not.toBeDisabled());
        if (inline) expect(screen.getByRole("alert")).toHaveTextContent("凭据错误");
        else expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        expect(navigate).not.toHaveBeenCalled();
    });
});
