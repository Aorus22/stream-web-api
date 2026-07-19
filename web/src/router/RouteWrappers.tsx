import { Navigate, Outlet } from "react-router-dom";
import type { ReactNode } from "react";
import AppLayout from "../components/AppLayout";
import { useServer } from "../contexts/ServerContext";

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { serverUrl } = useServer();

  if (!serverUrl) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

export function LayoutWrapper() {
  return (
    <AppLayout>
      <Outlet />
    </AppLayout>
  );
}
