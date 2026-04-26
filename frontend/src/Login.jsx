import { useState } from 'react';
import { useAuth } from './AuthContext';
import { useNavigate } from 'react-router-dom'; // Добавили useNavigate

export default function Login() {
    const { login, register } = useAuth();
    const navigate = useNavigate(); // Инициализация
    const [isRegisterMode, setIsRegisterMode] = useState(false);

    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setLoading(true);
        try {
            let userRole = '';
            if (isRegisterMode) {
                userRole = await register(email, password);
            } else {
                userRole = await login(email, password);
            }

            // Редиректим в зависимости от полученной роли
            if (userRole === 'admin') {
                navigate('/admin');
            } else {
                navigate('/user');
            }

        } catch (err) {
            // Ошибка уже выводится через alert в AuthContext
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="flex items-center justify-center min-h-screen bg-gray-100">
            <div className="p-8 bg-white rounded-lg shadow-md w-96">
                <h1 className="text-2xl font-bold mb-2 text-center text-gray-800">Room Booking</h1>
                <p className="text-center text-gray-500 mb-6">
                    {isRegisterMode ? 'Создайте новый аккаунт' : 'Войдите в свою учетную запись'}
                </p>

                <form onSubmit={handleSubmit} className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
                        <input
                            type="email" required
                            value={email} onChange={(e) => setEmail(e.target.value)}
                            className="w-full border p-2 rounded focus:ring-2 focus:ring-blue-500 outline-none"
                            placeholder="user@example.com"
                        />
                    </div>

                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Пароль</label>
                        <input
                            type="password" required minLength={3}
                            value={password} onChange={(e) => setPassword(e.target.value)}
                            className="w-full border p-2 rounded focus:ring-2 focus:ring-blue-500 outline-none"
                            placeholder="******"
                        />
                    </div>

                    <button
                        type="submit" disabled={loading}
                        className={`w-full py-2 px-4 text-white font-semibold rounded shadow transition disabled:opacity-50 ${isRegisterMode ? 'bg-green-600 hover:bg-green-700' : 'bg-blue-600 hover:bg-blue-700'
                            }`}
                    >
                        {loading ? 'Загрузка...' : (isRegisterMode ? 'Зарегистрироваться' : 'Войти')}
                    </button>
                </form>

                <div className="mt-6 text-center text-sm text-gray-600 border-t pt-4">
                    {isRegisterMode ? 'Уже есть аккаунт?' : 'Ещё нет аккаунта?'}
                    <button
                        type="button" // Добавлено type="button" чтобы не сабмитить форму
                        onClick={() => setIsRegisterMode(!isRegisterMode)}
                        className="ml-2 text-blue-600 hover:underline font-medium"
                    >
                        {isRegisterMode ? 'Войти' : 'Зарегистрироваться'}
                    </button>
                </div>
            </div>
        </div>
    );
}