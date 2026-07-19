import { createBrowserRouter } from 'react-router-dom';
import Library from '../pages/Library/Page';
import VideoPlayer from '../pages/VideoPlayer/Page';
import Search from '../pages/Search/Page';
import Browse from '../pages/Browser/Page';
import MediaDetail from '../pages/MediaDetail/Page';
import ServerLogin from '../pages/ServerLogin/Page';
import { CustomProviderListPage } from '../pages/CustomProvider/ListPage';
import { CustomProviderEditorPage } from '../pages/CustomProvider/EditorPage';
import FullHtmlPreviewPage from '../pages/CustomProvider/FullHtmlPreview';
import { ProtectedRoute, LayoutWrapper } from './RouteWrappers';

export const router = createBrowserRouter([
    {
        path: '/login',
        element: <ServerLogin />,
    },
    {
        path: '/watch',
        element: (
            <ProtectedRoute>
                <VideoPlayer />
            </ProtectedRoute>
        ),
    },
    {
        path: '/custom-provider/preview/:encodedHtml',
        element: (
            <ProtectedRoute>
                <FullHtmlPreviewPage />
            </ProtectedRoute>
        ),
    },
    {
        path: '/',
        element: (
            <ProtectedRoute>
                <LayoutWrapper />
            </ProtectedRoute>
        ),
        children: [
            {
                index: true,
                element: <Browse />,
            },
            {
                path: 'dashboard',
                element: <Library />,
            },
            {
                path: 'library',
                element: <Library />,
            },
            {
                path: 'search',
                element: <Search />,
            },
            {
                path: 'custom-provider',
                element: <CustomProviderListPage />,
            },
            {
                path: 'custom-provider/new',
                element: <CustomProviderEditorPage />,
            },
            {
                path: 'custom-provider/edit/:id',
                element: <CustomProviderEditorPage />,
            },
            {
                path: 'browse',
                element: <Browse />,
            },
            {
                path: 'discover',
                element: <Browse />,
            },
            {
                path: 'movie/:id',
                element: <MediaDetail />,
            },
            {
                path: 'series/:id',
                element: <MediaDetail />,
            },
            {
                path: 'tv/:id',
                element: <MediaDetail />,
            },
        ],
    },
]);
