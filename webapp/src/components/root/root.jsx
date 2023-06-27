import React from 'react';
import PropTypes from 'prop-types';

import {
    makeStyleFromTheme,
    changeOpacity,
} from 'mattermost-redux/utils/theme_utils';

import FullScreenModal from '../modals/modals.jsx';

import './root.scss';
import AutocompleteSelector from '../user_selector/autocomplete_selector.tsx';

const PostUtils = window.PostUtils;

export default class Root extends React.Component {
    static propTypes = {
        visible: PropTypes.bool.isRequired,
        message: PropTypes.string.isRequired,
        postID: PropTypes.string.isRequired,
        close: PropTypes.func.isRequired,
        submit: PropTypes.func.isRequired,
        theme: PropTypes.object.isRequired,
        autocompleteUsers: PropTypes.func.isRequired,
    };
    constructor(props) {
        super(props);

        this.state = {
            message: null,
            sendTo: null,
            attachToThread: false,
            previewMarkdown: false,
        };
    }

    static getDerivedStateFromProps(props, state) {
        if (props.visible && state.message == null) {
            return {message: props.message};
        }
        if (!props.visible && (state.message != null || state.sendTo != null)) {
            return {
                message: null,
                sendTo: null,
                attachToThread: false,
                previewMarkdown: false,
            };
        }
        return null;
    }

    handleAttachChange = (e) => {
        const value = e.target.checked;
        if (value !== this.state.attachToThread) {
            this.setState({
                attachToThread: value,
            });
        }
    };

    submit = () => {
        const {submit, close, postID} = this.props;
        const {message, sendTo, attachToThread} = this.state;
        if (attachToThread) {
            submit(message, sendTo, postID);
        } else {
            submit(message, sendTo);
        }

        close();
    };

    render() {
        const {visible, theme, close} = this.props;

        if (!visible) {
            return null;
        }

        const {message} = this.state;

        const style = getStyle(theme);
        const activeClass = 'btn btn-primary';
        const inactiveClass = 'btn';
        const writeButtonClass = this.state.previewMarkdown ?
            inactiveClass :
            activeClass;
        const previewButtonClass = this.state.previewMarkdown ?
            activeClass :
            inactiveClass;

        return (
            <FullScreenModal
                show={visible}
                onClose={close}
            >
                <div
                    style={style.modal}
                    className='ToDoPluginRootModal'
                >
                    <h1>{'Добавить задачу'}</h1>
                    <div className='todoplugin-issue'>
                        <h2>{'Текст задачи'}</h2>
                        <div className='btn-group'>
                            <button
                                className={writeButtonClass}
                                onClick={() => {
                                    this.setState({previewMarkdown: false});
                                }}
                            >
                                {'Write'}
                            </button>
                            <button
                                className={previewButtonClass}
                                onClick={() => {
                                    this.setState({previewMarkdown: true});
                                }}
                            >
                                {'Preview'}
                            </button>
                        </div>
                        {this.state.previewMarkdown ? (
                            <div
                                className='todoplugin-input'
                                style={style.markdown}
                            >
                                {PostUtils.messageHtmlToComponent(
                                    PostUtils.formatText(this.state.message),
                                )}
                            </div>
                        ) : (
                            <textarea
                                className='todoplugin-input'
                                style={style.textarea}
                                value={message}
                                onChange={(e) =>
                                    this.setState({message: e.target.value})
                                }
                            />
                        )}
                    </div>
                    {this.props.postID && (
                        <div className='todoplugin-add-to-thread'>
                            <input
                                type='checkbox'
                                checked={this.state.attachToThread}
                                onChange={this.handleAttachChange}
                            />
                            <b>{' Добавить в канал'}</b>
                            <div className='help-text'>
                                {
                                    ' Выберите, чтобы Робот задач отвечал на цепочку, когда прикрепленная задача добавляется, изменяется или завершается.'
                                }
                            </div>
                        </div>
                    )}
                    <div>
                        <AutocompleteSelector
                            id='send_to_user'
                            loadOptions={this.props.autocompleteUsers}
                            onSelected={(selected) =>
                                this.setState({sendTo: selected?.username})
                            }
                            label={'Переслать пользователю'}
                            helpText={
                                'Выберите пользователя, если хотите отправить это задание.'
                            }
                            placeholder={''}
                            theme={theme}
                        />
                    </div>
                    <div className='todoplugin-button-container'>
                        <button
                            className={'btn btn-primary'}
                            style={
                                message ? style.button : style.inactiveButton
                            }
                            onClick={this.submit}
                            disabled={!message}
                        >
                            {'Добавить задачу'}
                        </button>
                    </div>
                    <div className='todoplugin-divider'/>
                    <div className='todoplugin-clarification'>
                        <div className='todoplugin-question'>
                            {'Что это делает?'}
                        </div>
                        <div className='todoplugin-answer'>
                            {
                                'Добавление задачи добавит задачу в ваш список задач. Вы будете получать ежедневные напоминания о проблемах в Todo, пока не отметите их как завершенные.'
                            }
                        </div>
                        <div className='todoplugin-question'>
                            {'Чем это отличается от пометки поста?'}
                        </div>
                        <div className='todoplugin-answer'>
                            {
                                'Задачи отключены от сообщений. Вы можете создавать задачи из сообщений, но они не имеют никакой другой связи с сообщениями. Это позволяет создать более чистый список задач, который не зависит от истории сообщений или от того, что кто-то еще не удалил или не отредактировал сообщение.'
                            }
                        </div>
                    </div>
                </div>
            </FullScreenModal>
        );
    }
}

const getStyle = makeStyleFromTheme((theme) => {
    return {
        modal: {
            color: changeOpacity(theme.centerChannelColor, 0.88),
        },
        textarea: {
            backgroundColor: theme.centerChannelBg,
        },
        helpText: {
            color: changeOpacity(theme.centerChannelColor, 0.64),
        },
        button: {
            color: theme.buttonColor,
            backgroundColor: theme.buttonBg,
        },
        inactiveButton: {
            color: changeOpacity(theme.buttonColor, 0.88),
            backgroundColor: changeOpacity(theme.buttonBg, 0.32),
        },
        markdown: {
            minHeight: '149px',
            fontSize: '16px',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'end',
        },
    };
});
