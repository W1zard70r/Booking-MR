import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from './AuthContext';

export default function ProtectedRoute({ allowedRoles }) {
    const { token, role, loading } = useAuth();

    // Ждем, пока распарсится токен из localStorage
    if (loading) {
        return <div className="flex h-screen items-center justify-center">Загрузка...</div>;
    }

    // Если нет токена — на страницу логина
    if (!token) {
        return <Navigate to="/login" replace />;
    }

    // Если роль не совпадает с разрешенными — кидаем на нужную панель
    if (allowedRoles && !allowedRoles.includes(role)) {
        return <Navigate to={role === 'admin' ? "/admin" : "/user"} replace />;
    }

    // Если всё ок, рендерим дочерний маршрут
    return <Outlet />;
}