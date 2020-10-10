package gortsplib

import (
	"net/url"
)

// DialRead connects to the address and starts reading all tracks.
func DialRead(address string, proto StreamProtocol) (*ConnClient, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	conn, err := NewConnClient(ConnClientConf{Host: u.Host})
	if err != nil {
		return nil, err
	}

	_, err = conn.Options(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	tracks, _, err := conn.Describe(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if proto == StreamProtocolUDP {
		for _, track := range tracks {
			_, err := conn.SetupUDP(u, TransportModePlay, track, 0, 0)
			if err != nil {
				return nil, err
			}
		}

	} else {
		for _, track := range tracks {
			_, err := conn.SetupTCP(u, TransportModePlay, track)
			if err != nil {
				conn.Close()
				return nil, err
			}
		}
	}

	_, err = conn.Play(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// DialPublish connects to the address and starts publishing the tracks.
func DialPublish(address string, proto StreamProtocol, tracks Tracks) (*ConnClient, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	conn, err := NewConnClient(ConnClientConf{Host: u.Host})
	if err != nil {
		return nil, err
	}

	_, err = conn.Options(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	_, err = conn.Announce(u, tracks)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if proto == StreamProtocolUDP {
		for _, track := range tracks {
			_, err = conn.SetupUDP(u, TransportModeRecord, track, 0, 0)
			if err != nil {
				conn.Close()
				return nil, err
			}
		}

	} else {
		for _, track := range tracks {
			_, err = conn.SetupTCP(u, TransportModeRecord, track)
			if err != nil {
				conn.Close()
				return nil, err
			}
		}
	}

	_, err = conn.Record(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}
