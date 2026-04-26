import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './AuthContext';
import Login from './Login';
import AdminDashboard from './AdminDashboard';
import UserDashboard from './UserDashboard';
import ProtectedRoute from './ProtectedRoute';

// Компонент для корневого URL (/) - решает куда кинуть пользователя
const RootRedirect = () => {
  const { token, role, loading } = useAuth();
  if (loading) return null;
  if (!token) return <Navigate to="/login" replace />;
  return <Navigate to={role === 'admin' ? "/admin" : "/user"} replace />;
};

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          {/* Публичный роут */}
          <Route path="/login" element={<Login />} />

          {/* Редирект с корня */}
          <Route path="/" element={<RootRedirect />} />

          {/* Защищенные роуты ТОЛЬКО для админа */}
          <Route element={<ProtectedRoute allowedRoles={['admin']} />}>
            <Route path="/admin" element={<AdminDashboard />} />
          </Route>

          {/* Защищенные роуты для юзера (можно пускать и админа, если хочешь) */}
          <Route element={<ProtectedRoute allowedRoles={['user']} />}>
            <Route path="/user" element={<UserDashboard />} />
          </Route>

          {/* Обработка несуществующих страниц */}
          <Route path="*" element={<Navigate to="/" />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;