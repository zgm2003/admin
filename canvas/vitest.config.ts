import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

export default mergeConfig(
    viteConfig,
    defineConfig({
        test: {
            testTimeout: 30_000,
            projects: [
                {
                    extends: true,
                    test: {
                        name: "unit",
                        environment: "node",
                        include: ["tests/*.test.ts", "tests/store/auth.test.ts"],
                    },
                },
                {
                    extends: true,
                    test: {
                        name: "browser",
                        environment: "jsdom",
                        setupFiles: ["./tests/setup.ts"],
                        include: ["tests/**/*.test.{ts,tsx}"],
                        exclude: ["tests/*.test.ts", "tests/store/auth.test.ts"],
                    },
                },
            ],
        },
    }),
);
