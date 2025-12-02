import { useAuth } from "../hooks/useAuth";

type AuthModule = typeof import("../hooks/useAuth");

jest.mock("../hooks/useAuth");

export type AuthState = ReturnType<typeof useAuth>;
export type AuthUser = NonNullable<AuthState["user"]>;

export const mockUseAuth = useAuth as jest.MockedFunction<AuthModule["useAuth"]>;

export const buildAuthUser = (overrides: Partial<AuthUser> = {}): AuthUser => ({
  id: "viewer-1",
  displayName: "Viewer",
  email: "viewer@example.com",
  roles: [],
  ...overrides,
});

export const viewerUser = buildAuthUser();
export const viewerTwoUser = buildAuthUser({
  id: "viewer-2",
  displayName: "Viewer Two",
  email: "viewer2@example.com",
});
export const adminUser = buildAuthUser({
  id: "admin-1",
  displayName: "Admin",
  email: "admin@example.com",
  roles: ["admin"],
});
export const creatorUser = buildAuthUser({
  id: "creator-1",
  displayName: "Creator",
  email: "creator@example.com",
  roles: ["creator"],
});
export const ownerUser = buildAuthUser({
  id: "owner-1",
  displayName: "Owner",
  email: "owner@example.com",
  roles: ["creator"],
});

const baseAuthState = (): Omit<AuthState, "user"> => ({
  loading: false,
  error: undefined,
  signIn: jest.fn(),
  signOut: jest.fn(),
});

export const guestAuthState = (): AuthState => ({
  ...baseAuthState(),
  user: undefined,
});

export const signedInAuthState = (user: AuthUser = viewerUser): AuthState => ({
  ...baseAuthState(),
  user,
});

export const buildAuthState = (overrides: Partial<AuthState> = {}): AuthState => ({
  ...baseAuthState(),
  user: viewerUser,
  ...overrides,
});

export const mockAuthenticatedUser = (overrides: Partial<AuthUser> = {}): AuthUser => {
  const user = buildAuthUser(overrides);
  mockUseAuth.mockReturnValue(signedInAuthState(user));
  return user;
};

export const mockAnonymousUser = () => {
  mockUseAuth.mockReturnValue(guestAuthState());
};
