import { render, RenderOptions } from "@testing-library/react";
import React, { AnchorHTMLAttributes, MouseEvent, PropsWithChildren, ReactElement, Ref } from "react";

export * from "./auth";

export const mockRouter = {
  back: jest.fn(),
  forward: jest.fn(),
  prefetch: jest.fn(),
  push: jest.fn(),
  refresh: jest.fn(),
  replace: jest.fn(),
};

let pathname = "/";

export const setMockPathname = (nextPathname: string) => {
  pathname = nextPathname;
};

export const resetRouterMocks = () => {
  mockRouter.back.mockReset();
  mockRouter.forward.mockReset();
  mockRouter.prefetch.mockReset();
  mockRouter.push.mockReset();
  mockRouter.refresh.mockReset();
  mockRouter.replace.mockReset();
  pathname = "/";
};

jest.mock("next/link", () => {
  const React = require("react");
  return React.forwardRef(function MockLink(
    { children, onClick, ...props }: AnchorHTMLAttributes<HTMLAnchorElement>,
    ref: Ref<HTMLAnchorElement>
  ) {
    return React.createElement(
      "a",
      {
        ...props,
        ref,
        onClick: (event: MouseEvent<HTMLAnchorElement>) => {
          event.preventDefault();
          onClick?.(event);
        },
      },
      children
    );
  });
});

jest.mock("next/navigation", () => ({
  useRouter: () => mockRouter,
  usePathname: () => pathname,
}));

jest.mock("next/image", () => ({
  __esModule: true,
  default: ({ alt, ...props }: { alt?: string }) => <img alt={alt} {...props} />,
}));

export function renderWithProviders(ui: ReactElement, options?: RenderOptions) {
  const Wrapper = ({ children }: PropsWithChildren<unknown>) => <>{children}</>;

  return render(ui, { wrapper: Wrapper, ...options });
}
