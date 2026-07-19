import { createContext, useContext, useEffect, useState, type ReactNode } from "react"

type ServerContextType = {
    serverUrl: string | null
    setServerUrl: (url: string | null) => void
    isConnected: boolean
    checkConnection: () => Promise<boolean>
}

const ServerContextContext = createContext<ServerContextType | undefined>(undefined)

const STORAGE_KEY = "torrentstream-server-url"

export function ServerProvider({ children }: { children: ReactNode }) {
    const [serverUrl, setServerUrlState] = useState<string | null>(() => {
        return localStorage.getItem(STORAGE_KEY) || null
    })
    const [isConnected, setIsConnected] = useState(false)

    const checkConnection = async (): Promise<boolean> => {
        if (!serverUrl) {
            setIsConnected(false)
            return false
        }

        try {
            const response = await fetch(`${serverUrl}/api/torrents`, {
                method: "GET",
                signal: AbortSignal.timeout(5000)
            })
            const connected = response.ok
            setIsConnected(connected)
            return connected
        } catch (error) {
            setIsConnected(false)
            return false
        }
    }

    useEffect(() => {
        if (serverUrl) {
            checkConnection()
        } else {
            setIsConnected(false)
        }
    }, [serverUrl])

    const setServerUrl = (url: string | null) => {
        if (url) {
            localStorage.setItem(STORAGE_KEY, url)
            setServerUrlState(url)
        } else {
            localStorage.removeItem(STORAGE_KEY)
            setServerUrlState(null)
        }
    }

    return (
        <ServerContextContext.Provider value={{ serverUrl, setServerUrl, isConnected, checkConnection }}>
            {children}
        </ServerContextContext.Provider>
    )
}

export const useServer = () => {
    const context = useContext(ServerContextContext)
    if (context === undefined) {
        throw new Error("useServer must be used within a ServerProvider")
    }
    return context
}
