import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import type { ReactNode } from "react";

const { fetchMock } = vi.hoisted(() => {
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);
    return { fetchMock };
});

// Version checks and theme animation are unrelated to the session controls.
vi.mock("@/components/layout/version-release-modal", () => ({ VersionReleaseModal: () => null }));
vi.mock("@/components/ui/animated-theme-toggler", () => ({ AnimatedThemeToggler: () => null }));
vi.mock("@/components/layout/github-link", () => ({ GitHubLink: () => null }));
vi.mock("antd", () => ({ Tooltip: ({ children }: { children: ReactNode }) => children }));

import { UserStatusActions } from "@/components/layout/user-status-actions";
import { authStore, permissionStore } from "@/store";

const profilePage = {
    code: "user:profile:view",
    menuType: "page",
    path: "/user/profile",
    componentPath: "user/profile",
    i18nKey: "layout.user.profile",
    icon: null,
    isHidden: 1,
    children: [],
};

async function grant(permissionCodes: string[], menuTree: unknown[]) {
    fetchMock.mockResolvedValueOnce(
        new Response(
            JSON.stringify({
                code: 0,
                data: { roleCodes: [], permissionCodes, menuTree },
                message: "ok",
            }),
        ),
    );
    await act(() => permissionStore.load(true));
}

describe("Canvas session controls", () => {
    beforeEach(() => {
        fetchMock.mockReset();
        authStore.markAnonymous();
        permissionStore.reset();
    });

    it("shows the hidden profile page only after an explicit page grant and removes it after reset", async () => {
        render(
            <MemoryRouter>
                <UserStatusActions />
            </MemoryRouter>,
        );
        expect(screen.queryByRole("link", { name: "个人中心" })).not.toBeInTheDocument();

        await grant(["user:profile:update"], []);
        expect(screen.queryByRole("link", { name: "个人中心" })).not.toBeInTheDocument();

        await grant(["user:profile:view"], [profilePage]);
        expect(screen.getByRole("link", { name: "个人中心" })).toHaveAttribute("href", "/user/profile");

        act(() => permissionStore.reset());
        expect(screen.queryByRole("link", { name: "个人中心" })).not.toBeInTheDocument();
    });

    it("prevents repeated logout requests while the session is being revoked", async () => {
        let finish!: (response: Response) => void;
        fetchMock.mockImplementationOnce(
            () =>
                new Promise<Response>((resolve) => {
                    finish = resolve;
                }),
        );
        render(
            <MemoryRouter>
                <UserStatusActions />
            </MemoryRouter>,
        );
        const button = screen.getByRole("button", { name: "退出登录" });
        fireEvent.click(button);
        expect(button).toBeDisabled();
        fireEvent.click(button);
        expect(fetchMock).toHaveBeenCalledTimes(1);
        await act(async () => finish(new Response(JSON.stringify({ code: 0, data: {}, message: "ok" }))));
        await waitFor(() => expect(button).not.toBeDisabled());
    });

    it("clears the local session without an unhandled rejection when logout fails", async () => {
        authStore.setAuthenticated(
            { accessToken: "active-token", expiresIn: 300, isNewUser: false, passwordSetRequired: false },
            {
                userId: 7,
                username: "canvas-user",
                email: "user@example.com",
                phone: null,
                avatar: "",
                passwordSetRequired: false,
            },
        );
        fetchMock.mockRejectedValueOnce(new TypeError("network unavailable"));
        render(
            <MemoryRouter>
                <UserStatusActions />
            </MemoryRouter>,
        );
        fireEvent.click(screen.getByRole("button", { name: "退出登录" }));
        await waitFor(() => expect(authStore.state.status).toBe("anonymous"));
        expect(authStore.state.user).toBeNull();
    });
});
